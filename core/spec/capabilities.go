// SPDX-License-Identifier: GPL-3.0-or-later

package spec

// Capabilities describes everything generic code (UI, validation, clone
// service) needs to know about what a specific radio can do, in neutral
// terms. A driver package (e.g. the FT-710 driver) constructs the one
// Capabilities value for its radio; nothing in this package or its callers
// hardcodes facts about any particular radio model.
type Capabilities struct {
	// Model is the radio's display name, e.g. "FT-710".
	Model string
	// CATID is the radio's 4-character CAT ID answer, e.g. "0800".
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
	// Bauds lists the CAT serial baud rates this radio supports.
	Bauds []int
	// DefaultBaud is this radio's factory-default CAT baud rate.
	DefaultBaud int
	// MinFreqHz is the lowest storable frequency (radio tuning range
	// floor).
	MinFreqHz uint32
	// MaxFreqHz is the highest storable frequency (radio tuning range
	// ceiling).
	MaxFreqHz uint32
	// RequiredSlots lists canonical slot wire forms that must never be
	// empty, e.g. FT-710 M-01 ("001"). This is distinct from Bank.NoBlank,
	// which means "every slot in this bank must be populated" (used for
	// PMS): RequiredSlots is for individual slots.
	RequiredSlots []string
	// ShiftOptions lists the repeater shift vocabulary this radio's wire
	// protocol expresses, in the UI's preferred order, e.g. {Value: "PLUS",
	// Direction: ShiftUp}. Typically built from StandardShiftOptions().
	ShiftOptions []ShiftOption
	// CTCSSStates lists the CTCSS state vocabulary this radio's wire
	// protocol expresses, each paired with whether that state requires a
	// known CTCSS tone to accompany it (see ToneState.RequiresTone).
	// Typically built from StandardCTCSSStates().
	CTCSSStates []ToneState
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
