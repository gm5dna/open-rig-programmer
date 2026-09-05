// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"fmt"
)

// The two memory frame lengths, and the family's shared 50-byte grid.
//
// ONE GRID, BOTH DIRECTIONS, BOTH RADIOS. The MR answer and the MW Set
// frame are the same 50 bytes in the same order on both books (590:1440-1461
// and 590:1518-1536; 480:923-943 and 480:955-976), which is why the offsets
// below are package constants and only their MEANINGS are per-layout.
//
// MW EMITS EXACTLY RecordLen AND THE BUILDER REFUSES ANY OTHER WIDTH. That
// is what keeps the erase form from escaping by accident: 590:1579-1581
// describes a short MW that erases the channel, its length is a reading
// rather than a printed number (A5, erratum E19), and this milestone never
// builds it (decision 8).
const (
	// RecordLen is the MR answer and the MW Set frame: 50 bytes.
	RecordLen = 50
	// MRReadLen is the MR read request, "M R P1 P2 P3 P3 ;": 7 bytes
	// (590:1442, 480:918). The 590SG chart prints its terminator cell as
	// ':' — erratum E1, doc.go — and the frame is a ';' frame like every
	// other.
	MRReadLen = 7
)

// Byte offsets (0-indexed) into the 50-byte grid, with each book's own
// 1-indexed positions in the comment so a reader can put these beside the
// charts without arithmetic.
const (
	recPrefixOff   = 0  // positions 1-2, "MR" or "MW"
	recP1Off       = 2  // position 3,     P1
	recP2Off       = 3  // position 4,     P2
	recP3Off       = 4  // positions 5-6,  P3
	recFreqOff     = 6  // positions 7-17, P4
	recModeOff     = 17 // position 18,    P5
	recByte19Off   = 18 // position 19,    P6
	recToneModeOff = 19 // position 20,    P7
	recToneOff     = 20 // positions 21-22, P8
	recCTCSSOff    = 22 // positions 23-24, P9
	recByte3940Off = 38 // positions 39-40, P14
	recByte28Off   = 27 // position 28,    P11
	recNameOff     = 41 // positions 42-49, P16
	recByte41Off   = 40 // position 41,    P15
	recTermOff     = 49 // position 50,    ';'
)

// Field widths for the offsets above.
const (
	recP3Digits     = 2
	recFreqDigits   = 11
	recToneDigits   = 2
	recCTCSSDigits  = 2
	recByte3940Len  = 2
	recNameLen      = 8
	recPrefixLen    = 2
	recTailLen      = 1 // the terminator
	recFixedNonBody = recPrefixLen + recTailLen
)

// The empty-channel window: P4 through P15, positions 7 to 41 inclusive.
//
// "If the selected channel is empty, P4 ~ P15 will be 0 and P16 will be
// blank." (590:1492-1493). THE TEST IS THIS WHOLE WINDOW, not P5 alone: a
// parser that read a zero mode nibble as a failure would refuse at the first
// empty channel and fail a whole-radio read of a fresh radio. That is A18a,
// which is documentary fact rather than an assumption.
const (
	recEmptyLoOff = recFreqOff   // position 7
	recEmptyHiOff = recByte41Off // position 41
)

// maxSlotNumber is the largest number P2 and P3 can carry between them:
// three digits, one cell for P2 and two for P3 on both charts' own rulers
// (590:1518-1519, 480:952-955). It bounds a LAYOUT'S declared slot space; it
// is not a claim that any radio has 1000 channels — and in particular the
// 480 prints "Always 0" at P2 (480:953) and a two-digit channel number at P3
// (480:955), which is maxFixedZeroSlot's datum below, not this one's.
const maxSlotNumber = 999

// maxFixedZeroSlot is the largest slot number a row whose byte 4 is a
// printed constant can name, and it is READ FROM THE SAME CHART AS ITS
// DATUM: where 480:953 prints "Always 0 for the TS-480." for P2, 480:955
// prints "00 ~ 99: Memory channel number" for P3. A row with no hundreds
// digit has only those two digits, so its slot space stops at 99.
//
// NewLayout refuses P2FixedZero alongside any range above it
// (validateSlots), which is what keeps slotWire from having to render a
// three-digit number into a two-digit field —
// TestNewLayout_RefusesAFixedZeroByte4AboveTheTwoDigitCeiling is the pin.
const maxFixedZeroSlot = 99

// MaxRecordFreqHz is the widest value the 11-digit P4 field can carry
// (590:1541-1543, 480:957).
//
// IT IS A FIELD WIDTH AND NOTHING MORE. Neither book prints a tuning range
// for MR/MW P4 — only "11 digits" — which is A17, and A17's own lift is a
// document read rather than a wire observation because a width is not a
// range. Nothing here says a radio will accept 99,999,999,999 Hz.
const MaxRecordFreqHz = 99_999_999_999

// ErrOutOfDomain is the sentinel every OutOfDomainError wraps.
var ErrOutOfDomain = errors.New("kw: value outside a wire field's domain")

// OutOfDomainError reports a value this codec cannot put on the wire.
//
// ITS MESSAGE IS ABOUT THE FIELD WIDTH, BECAUSE THAT IS WHAT IT KNOWS. The
// only bound this codec has for a memory frequency is the printed digit
// count; the radios' tuning ranges are unprinted (A17). Saying "outside the
// radio's range" would be an invention, and saying nothing would leave a
// caller guessing which of its fields was refused.
type OutOfDomainError struct {
	// Field names the parameter, as the books letter it.
	Field string
	// Value is what the caller asked for.
	Value uint64
	// Digits is the printed field width, and Max the widest value it holds.
	Digits int
	Max    uint64
}

// Error implements the error interface.
func (e *OutOfDomainError) Error() string {
	return fmt.Sprintf("kw: %d does not fit %s, whose printed width is %d digits and whose widest value is therefore %d — this names the FIELD WIDTH, not any radio's tuning range, which neither book prints (A17)", e.Value, e.Field, e.Digits, e.Max)
}

// Unwrap lets errors.Is(err, ErrOutOfDomain) match.
func (e *OutOfDomainError) Unwrap() error { return ErrOutOfDomain }

// ScanHalf names which of a section-defined channel's two frequencies a
// record carries.
//
// A section channel holds a START and an END frequency reached by the same
// slot number with different P1 bytes (590:1529-1531), so a record naming
// such a slot is incomplete until it says which half it is.
type ScanHalf int

// The two halves, plus the value an ordinary memory slot carries.
const (
	// ScanHalfNone is what a slot outside SlotScan carries: there is one
	// frequency and no half to name.
	ScanHalfNone ScanHalf = iota
	// ScanLower is the START frequency, written and read with P1 = '0'.
	ScanLower
	// ScanUpper is the END frequency, written and read with P1 = '1'.
	ScanUpper
)

// String renders h for refusals, logs and slot identifiers.
func (h ScanHalf) String() string {
	switch h {
	case ScanLower:
		return "L"
	case ScanUpper:
		return "U"
	default:
		return ""
	}
}

// Slot is one memory slot resolved against a layout's slot space: its
// number, the class that number falls in, and — for a section-defined
// channel — which of its two frequencies is meant.
//
// THE ZERO VALUE IS INVALID AND CARRIES NO CLASS, which matters because
// channel 000 is a real slot on the 590 pair: a Slot cannot be tested for
// emptiness by its number.
type Slot struct {
	number int
	class  SlotClass
	half   ScanHalf
}

// Number is the slot's channel number.
func (s Slot) Number() int { return s.number }

// Class is the class the constructing layout resolved the number to.
func (s Slot) Class() SlotClass { return s.class }

// Half is which of a section channel's two frequencies this slot names, and
// ScanHalfNone for every other class.
func (s Slot) Half() ScanHalf { return s.half }

// IsZero reports whether s was never constructed by a layout.
func (s Slot) IsZero() bool { return s.class == SlotClassInvalid }

// String is the slot's identifier: three digits, with an "L" or "U" suffix
// on a section-defined channel — "042", "100L", "100U".
func (s Slot) String() string {
	if s.IsZero() {
		return "<invalid slot>"
	}
	return fmt.Sprintf("%03d%s", s.number, s.half)
}

// P1 is the record's P1 byte for this slot, DERIVED FROM THE SLOT'S CLASS
// AND NEVER FROM THE CHANNEL'S SPLIT STATE.
//
// THIS IS M9, AND IT IS A DATA-LOSS GUARD RATHER THAN A FORMATTING RULE.
// The books give P1 two jobs at once. On an ordinary memory channel it
// selects simplex or split on the 590 pair (590:1519-1520 is the legend,
// 590:1521-1523 the sentence that carries the consequence) and the RX or TX
// frequency on the 480 (480:951); on a section-defined channel it selects
// the START or the END frequency instead — "set parameter P1 to 0 to enter
// the Start frequency, then set P1 to 1 to set the End frequency"
// (590:1529-1531). A builder that took P1 from a channel's split state would
// write '0' into a section channel's END slot, putting the START frequency
// where the user had edited the END: a silent data loss, and exactly the
// class of error decision 11 exists to prevent.
//
// THE 480 PRINTS THE SAME OVERLOAD AND THIS PROGRAMME DOES NOT USE IT.
// "Memory channel 90 ~ 99: P1=0 (start frequency), P1=1 (end frequency)"
// (480:986-987) is really printed, but the spec rules the other way for that
// radio: it gets no scan bank, its 90-99 are ordinary memories that also
// answer a second frame, and "the P1=1 half of 90-99 is unreachable through
// this programme" (spec :711-718, decision 15, matrix erratum M-E2).
// layout480 declares a flat 000-099 SlotMemory accordingly, so SlotScan is a
// 590-only class here and the citation above is context, not a rule this
// codec applies to the 480.
//
// THE '0' THIS CODEC EMITS ON AN ORDINARY MEMORY SLOT FLATTENS AN EXISTING
// SPLIT, and that is a wire consequence a Stage 2 author reading only this
// package would otherwise miss. "After setting P1 to 0, the channel becomes
// a simplex channel, even if it was already a split channel." (590:1521-1523
// — the legend at 590:1519-1520 says only "0: Simplex / 1: Split" and does
// NOT say this). Nothing in this package can build a split write, because P1
// comes from the slot's class; the REFUSAL that keeps a driver from
// overwriting a split channel it did not read is spec decision 11's, keyed
// on a Known TxFreqHz, and it lives in the driver's write path rather than
// here.
//
// So: '0' for an ordinary memory slot and for a section channel's LOWER
// half; '1' for a section channel's UPPER half.
//
// SlotExtension takes '0' as well. Nothing in either book explains what an
// extension channel IS (A11), so no second frequency is claimed for one and
// no third P1 value exists to give it; Stuart ruled on 05/09/2026 that the
// ten SG slots are omitted from the driver's published banks until A11
// lifts, so nothing this programme ships reaches this arm.
//
// TestBuildMWSet_P1IsDerivedFromTheSlotClassNotTheSplitState pins all three
// arms, and its SCAN 'U' case is the red proof that the upper half is not
// simply '0': an implementation returning '0' unconditionally passes every
// other case in this package and fails only there.
func (s Slot) P1() byte {
	if s.class == SlotScan && s.half == ScanUpper {
		return '1'
	}
	return '0'
}

// NewSlot resolves number against this layout's slot space, with half
// naming which frequency of a section-defined channel is meant.
//
// A section channel MUST name a half and every other class must not: a
// record for a slot that holds two frequencies is incomplete without one,
// and a half on a slot that holds one would be a claim the books do not
// make.
//
// A zero Layout has no slot space and so admits no slot at all.
func (l Layout) NewSlot(number int, half ScanHalf) (Slot, error) {
	if !l.Configured() {
		return Slot{}, newParseError(nil, "slot %d: this layout is unconfigured and describes no radio's slot space", number)
	}
	class := l.classOf(number)
	if class == SlotClassInvalid {
		return Slot{}, newParseError(nil, "slot %d is outside the %s's slot space %s", number, l.model, l.slotSpaceText())
	}
	if class == SlotScan {
		if half != ScanLower && half != ScanUpper {
			return Slot{}, newParseError(nil, "slot %d is a section-defined channel on the %s, which holds a start and an end frequency (590:1529-1531), so a record naming it must say which half it carries", number, l.model)
		}
	} else if half != ScanHalfNone {
		return Slot{}, newParseError(nil, "slot %d is %v on the %s, which holds one frequency, so it has no half to name", number, class, l.model)
	}
	return Slot{number: number, class: class, half: half}, nil
}

// slotSpaceText renders this layout's slot ranges for a refusal.
func (l Layout) slotSpaceText() string {
	if len(l.slots) == 0 {
		return "(empty)"
	}
	out := ""
	for i, r := range l.slots {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%03d-%03d %v", r.Lo, r.Hi, r.Class)
	}
	return out
}

// Record is the decoded content of one 50-byte MR answer or MW Set frame.
//
// THE RAW-BYTE FIELDS ARE RAW ON PURPOSE. Byte 19, byte 28, bytes 39-40 and
// byte 41 mean different things on the two books, and three of them are a
// printed constant on one radio and a live field on another. A field named
// for one radio's reading — "DataMode bool" — would carry a TS-480 record's
// LOCKOUT under a name that says data mode, and every consumer would then be
// wrong in the same direction. The layout is what says which byte is which,
// and the driver is what maps a byte to a neutral field.
type Record struct {
	// Slot is the channel this record is for, resolved to its class. P1 is
	// derived from it; see Slot.P1.
	Slot Slot

	// FreqHz is P4, the 11-digit frequency in Hz (590:1541-1543, 480:957).
	FreqHz uint64

	// Mode is P5, the MD nibble (590:1544-1545, 480:959).
	Mode Mode

	// Byte19 is P6 as a wire byte: the data mode on the 590 pair
	// (590:1546-1548), the channel lockout on the 480 (480:962).
	Byte19 byte

	// ToneMode is P7 (590:1549-1553, 480:964).
	ToneMode ToneMode

	// ToneIndex is P8 and CTCSSIndex P9: INDEPENDENT TX and RX tone
	// indices, which is what makes the 590 pair's "3: Cross Tone ON"
	// expressible at all (590:1554-1557, 480:966-969).
	ToneIndex  int
	CTCSSIndex int

	// Byte28 is P11 as a wire byte: FILTER A/B on the 590 pair
	// (590:1560-1563) and a printed constant on the 480 (480:973).
	Byte28 byte

	// Byte3940 is P14 as its two wire bytes: the FM Normal/Narrow flag on
	// the 590 pair (590:1569-1571) and the tuning step index on the 480
	// (480:979).
	Byte3940 string

	// Byte41 is P15 as a wire byte: the channel lockout on the 590 pair
	// (590:1572-1574) and a printed constant on the 480 (480:982).
	Byte41 byte

	// Name is P16, right-trimmed of the spaces a write pads it with — A1,
	// which is assumed rather than printed and whose lift is a write-then-
	// read on each registry row.
	Name string

	// Empty reports that the frame carried P4-P15 all zero: the empty
	// channel of 590:1492-1493, which is A18a and documentary fact. It is
	// set by the parser only; a builder never writes an empty channel.
	Empty bool

	// AnswerP1 is P1 EXACTLY AS READ, and it is a parser output rather than
	// a builder input. BuildMWSet derives P1 from the slot's class (M9,
	// Slot.P1) and refuses a record whose AnswerP1 disagrees, so a record
	// read from one slot cannot be written back to another half by
	// accident.
	//
	// It is zero — not an ASCII digit — on a record the caller built.
	AnswerP1 byte
}

// emptyName is A3's reading of "P16 will be blank" (590:1492-1493): the
// eight bytes of P16, every one a space. It is stated once, beside the
// window predicate it accompanies, because the two are the two halves of one
// sentence and a second copy of either would be one edit from disagreeing.
const emptyName = "        " // recNameLen spaces

// isEmptyWindow reports whether every byte of P4-P15 in frame is '0', which
// is the empty-channel test of 590:1492-1493 (A18a). It is HALF the
// sentence: the other half is P16, which parseRecordFrame requires to be
// emptyName.
func isEmptyWindow(frame []byte) bool {
	for _, b := range frame[recEmptyLoOff : recEmptyHiOff+1] {
		if b != '0' {
			return false
		}
	}
	return true
}
