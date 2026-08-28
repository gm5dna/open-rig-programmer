// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "fmt"

// Capabilities describes everything generic code (UI, validation, clone
// service) needs to know about what a specific radio can do, in neutral
// terms. A driver package (e.g. the FT-710 driver) constructs the one
// Capabilities value for its radio; nothing in this package or its callers
// hardcodes facts about any particular radio model.
type Capabilities struct {
	// Model is the radio's display name, e.g. "FT-710".
	Model string
	// CATID is a CAT ID (four hex digits on Yaesu; the CI-V address,
	// optionally with a recorded token, on Icom), e.g. "0800" for an
	// FT-710 or "98" for an IC-7610's static value. See
	// core/driver.Identity.CATID for the tier's casing convention.
	CATID string
	// Banks lists every memory-slot family this radio supports.
	Banks []Bank
	// Modes lists display-name modes in the UI's preferred order, e.g.
	// "LSB", "USB", ...
	Modes []string
	// TagLen is the maximum tag/name length in characters, e.g. 12.
	TagLen int
	// ClarMaxHz is the maximum clarifier offset in hertz, e.g. 9990.
	ClarMaxHz int
	// ClarStepHz is the clarifier step size in hertz, e.g. 10.
	ClarStepHz int
	// CTCSSTones is the CTCSS tone table this radio uses, indexed by CAT
	// tone number. Typically built from StandardCTCSSTones(), e.g.
	// tones := StandardCTCSSTones(); CTCSSTones: tones[:].
	CTCSSTones []Tone
	// CTCSSToneRange is the ALTERNATIVE to CTCSSTones for a radio whose
	// tone field is a NUMBER rather than an index into a chart — every
	// CI-V model in the Icom tier. Nil for a radio that declares a list
	// (all four Yaesu models); a radio declares one or the other, never
	// both, and Validate refuses both.
	//
	// A POINTER, so presence is the declaration — see ToneRange. Ask
	// AdmitsTone rather than reading either field: it is the one
	// predicate that knows about both shapes, and it fails closed when a
	// radio declares neither.
	CTCSSToneRange *ToneRange
	// Bauds lists the CAT serial baud rates this radio supports.
	Bauds []int
	// DefaultBaud is this radio's factory-default CAT baud rate.
	DefaultBaud int
	// MinFreqHz is the lowest storable frequency (radio tuning range
	// floor).
	//
	// uint64, not uint32, since the Icom tier (design D4, adjudication
	// 5): the IC-905 reaches 10 GHz and a uint32 caps at 4.29 GHz. The
	// widening is invisible to every existing caller — the four Yaesu
	// profiles' literals are untyped constants — and JSON representation
	// is unchanged (both are plain numbers).
	MinFreqHz uint64
	// MaxFreqHz is the highest storable frequency (radio tuning range
	// ceiling). uint64 for the reason MinFreqHz gives.
	MaxFreqHz uint64
	// RequiredSlots lists canonical slot wire forms that must never be
	// empty, e.g. FT-710 M-01 ("001"). This is distinct from Bank.NoBlank,
	// which means "every slot in this bank must be populated" (used for
	// PMS): RequiredSlots is for individual slots.
	RequiredSlots []string
	// ShiftOptions lists the repeater shift vocabulary this radio's wire
	// protocol expresses, in the UI's preferred order, e.g. {Value: "PLUS",
	// Direction: ShiftUp}. Typically built from StandardShiftOptions().
	// Every entry's Direction must be one of ShiftNone/ShiftUp/ShiftDown
	// (never the zero value, ShiftUnspecified) — see Validate.
	ShiftOptions []ShiftOption
	// CTCSSStates lists the CTCSS state vocabulary this radio's wire
	// protocol expresses, each paired with the semantic fact of whether
	// that state requires a known CTCSS tone to accompany it (see
	// ToneState.RequiresTone). Typically built from StandardCTCSSStates().
	// Every entry's Semantics must be one of ToneOff/ToneEncode/
	// ToneEncodeDecode (never the zero value, ToneSemanticsUnspecified) —
	// see Validate.
	CTCSSStates []ToneState

	// The vocabularies the Icom tier adds (design D4). EVERY ONE OF THEM
	// IS EMPTY on the four Yaesu NEWCAT models registered before that
	// tier, and empty is not a gap to be filled in later by a default:
	// it is the positive statement "this radio expresses no such
	// vocabulary", and it is what every capability-keyed check in
	// core/codeplug and core/csvio tests before it runs. That is how the
	// Yaesu outputs stay byte-identical.
	//
	// The two families' vocabularies never coexist on one model: a radio
	// supplies ShiftOptions+CTCSSStates or DuplexOptions+ToneModes,
	// never both. Validate enforces that at least one of each PAIR is
	// present, rather than demanding the Yaesu half unconditionally as
	// it did before this tier.

	// DuplexOptions lists the FieldDuplex vocabulary this radio's wire
	// protocol expresses, in the UI's preferred order, each paired with
	// its DuplexDirection. Every entry's Direction must be one of
	// DuplexOff/DuplexUp/DuplexDown (never the zero value,
	// DuplexUnspecified) — see Validate.
	DuplexOptions []DuplexOption
	// ToneModes lists the FieldToneMode vocabulary this radio expresses,
	// in the UI's preferred order, each paired with its
	// ToneModeSemantics. Every entry's Semantics must be one of the five
	// declared, meaningful constants (never ToneModeUnspecified) — see
	// Validate.
	ToneModes []ToneMode
	// DTCSPolarities lists the FieldDTCSPolarity vocabulary this radio
	// expresses, e.g. {"NN", "NR", "RN", "RR"}. Plain strings: unlike
	// duplex and tone mode, generic code needs no semantic fact about a
	// polarity beyond which spellings are legal.
	DTCSPolarities []string
	// DTCSCodes is the DTCS/DCS code table this radio uses, as the code
	// NUMBERS themselves (23 for "023"), strictly ascending. A Known
	// FieldDTCSCode outside this table can never have come from, or be
	// sendable to, this radio.
	DTCSCodes []int
	// Filters lists the FieldFilter vocabulary this radio expresses,
	// e.g. {"FIL1", "FIL2", "FIL3"}.
	Filters []string

	// TagCharset is the exact set of bytes this radio accepts in a
	// channel tag/name, as a string whose every byte is one legal
	// character. EMPTY means the pre-Icom family default: printable
	// ASCII 0x20-0x7E excluding ';' (the CAT terminator). Supplying it
	// is what makes the name-charset check capability-supplied rather
	// than a Yaesu wire rule restated in neutral code (design D4) — the
	// Icom manuals print a charset table that omits the space, which the
	// default rule would wrongly allow.
	TagCharset string
}

// TagByteOK reports whether b is a legal byte in a channel tag on this
// radio: a member of TagCharset when that is supplied, and otherwise
// printable ASCII 0x20-0x7E excluding ';'.
//
// The default arm is the family rule core/codeplug and core/csvio each
// carried as their own literal predicate before this tier; both now ask
// here, so a radio that supplies a charset is honoured by the model
// layer and the CHIRP importer alike, and one that does not behaves
// exactly as it always did. ';' is excluded by the default because it
// terminates a NEWCAT frame — a tag containing one could smuggle a
// second command onto the wire — and a radio supplying its own charset
// is trusted to have thought about its own terminator.
func (c Capabilities) TagByteOK(b byte) bool {
	if c.TagCharset == "" {
		return b >= 0x20 && b <= 0x7E && b != ';'
	}
	for i := 0; i < len(c.TagCharset); i++ {
		if c.TagCharset[i] == b {
			return true
		}
	}
	return false
}

// TagCharsetDescription names this radio's tag charset for a
// human-readable error message: the default rule's own wording when
// TagCharset is empty, and a quoted listing of the supplied set
// otherwise. Kept beside TagByteOK so a message can never describe a
// different rule from the one that rejected the byte.
func (c Capabilities) TagCharsetDescription() string {
	if c.TagCharset == "" {
		return "printable ASCII 0x20-0x7E, excluding ';'"
	}
	return fmt.Sprintf("this radio's tag charset %q", c.TagCharset)
}

// Bank returns the Bank with the given ID and true if this Capabilities
// includes it, or the zero Bank and false otherwise.
//
// The returned Bank is a DEFENSIVE COPY (see copyBank): its Slots and
// Fields are independently allocated, so a caller mutating either one —
// e.g. tweaking a FieldSupport entry to experiment, or appending a slot —
// can never reach back into this Capabilities value and corrupt what
// every other caller sees. This matters more than usual for Fields
// specifically: it is this project's hardware-write gate data
// (FieldSupport.CanWrite), so an aliasing bug here could silently make a
// field look writable (or unwritable) project-wide.
func (c Capabilities) Bank(id BankID) (Bank, bool) {
	for _, b := range c.Banks {
		if b.ID == id {
			return copyBank(b), true
		}
	}
	return Bank{}, false
}

// copyBank returns a defensive copy of b: Slots and Fields are freshly
// allocated and independently populated, so mutating the returned Bank
// (its Slots slice or its Fields map) can never be observed through b or
// any other copy of it. A nil Slots/Fields copies to nil (not an empty
// non-nil value), preserving the zero-value distinction callers may
// depend on (e.g. Bank() returning the zero Bank for an absent ID).
func copyBank(b Bank) Bank {
	if b.Slots != nil {
		slots := make([]string, len(b.Slots))
		copy(slots, b.Slots)
		b.Slots = slots
	}
	if b.Fields != nil {
		fields := make(map[Field]FieldSupport, len(b.Fields))
		for f, fs := range b.Fields {
			fields[f] = fs
		}
		b.Fields = fields
	}
	return b
}

// FieldSupport looks up the FieldSupport for f within bank. If bank is not
// present in this Capabilities, or is present but does not list f, it
// returns the zero FieldSupport (Read: Unsupported, Write: Unsupported) —
// callers do not need to special-case "unknown" separately from
// "unsupported": both mean the field cannot be trusted, and in particular
// CanWrite() is false either way.
func (c Capabilities) FieldSupport(bank BankID, f Field) FieldSupport {
	b, ok := c.Bank(bank)
	if !ok {
		return FieldSupport{}
	}
	return b.Fields[f]
}
