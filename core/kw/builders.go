// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "fmt"

// BuildMRRead builds the memory-channel read request for slot s: "M R P1 P2
// P3 P3 ;", seven bytes (590:1442, 480:918).
//
// The 590SG chart prints the terminator cell as ':' — erratum E1 — and the
// frame this builds is a ';' frame like every other in both books.
//
// P1 IS DERIVED FROM THE SLOT'S CLASS, exactly as it is on the write side.
// Reading a section channel's END frequency means P1='1' ("When reading the
// start frequency of a section defined channel, enter 0 for parameter P1.
// When reading the end frequency, enter 1.", 590:1445-1451), and a read that
// took P1 from anywhere else would return the wrong half of the pair and
// then store it under the half the caller asked for.
func (l Layout) BuildMRRead(s Slot) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "MR read: this layout is unconfigured and describes no radio")
	}
	if err := l.checkSlot("MR read", s); err != nil {
		return Command{}, err
	}
	frame := make([]byte, 0, MRReadLen)
	frame = append(frame, 'M', 'R')
	frame = append(frame, s.P1())
	wire, err := l.slotWire(s)
	if err != nil {
		return Command{}, err
	}
	frame = append(frame, wire...)
	frame = append(frame, ';')
	if len(frame) != MRReadLen {
		return Command{}, newParseError(frame, "MR read: built %d bytes, want exactly %d (590:1442, 480:918)", len(frame), MRReadLen)
	}
	return newCommand(frame), nil
}

// BuildMWSet builds the 50-byte memory-channel write for rec.
//
// IT EMITS EXACTLY RecordLen BYTES AND ITS OWN GATE REFUSES ANY OTHER WIDTH.
// That is what keeps the erase form from escaping by accident: 590:1579-1581
// describes a short MW that erases the channel specified by P2 and P3, its
// length is a reading rather than a printed number (A5, erratum E19), and
// this milestone never builds it (decision 8). The gate consults
// checkRecordLen, the same predicate the answer parser uses, so the two
// cannot drift apart.
//
// Every refusal below is a field this codec cannot put on the wire without
// inventing something. None of them is the DRIVER'S refusals — A22's blanket
// refusal of TS-480 channel writes and A23's refusal of non-FM 590 writes are
// decisions about which records a driver may offer this builder, taken where
// the neutral field vocabulary lives; this builder's job is that no frame it
// returns contains a byte no book describes.
func (l Layout) BuildMWSet(rec Record) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "MW set: this layout is unconfigured and describes no radio")
	}
	if err := l.checkSlot("MW set", rec.Slot); err != nil {
		return Command{}, err
	}

	// M9's second half. rec.AnswerP1 is a PARSER output; the builder derives
	// P1 from the slot's class. If a caller has carried a record from one
	// half of a section channel to the other without changing its slot, the
	// two disagree, and the write would put the START frequency in the END
	// slot: the silent data loss decision 11 exists to prevent.
	//
	// ON THIS MILESTONE'S PATHS THE REFUSAL IS LATENT RATHER THAN LIVE, and
	// that is worth saying because the message names only section-defined
	// channels. Plan P13 reads MR with P1=0 on every MEM slot and P1=1 only
	// on a SCAN 'U' slot — A8 and A9 are why — so a MEM record can only ever
	// carry AnswerP1='0', which is exactly what Slot.P1() derives for it, and
	// no MEM read on this milestone can reach this refusal. If a later path
	// ever reads a MEM slot answering P1='1', the message would be wrong as
	// well as surprising and should be widened before that path lands.
	if want := rec.Slot.P1(); rec.AnswerP1 != 0 && rec.AnswerP1 != want {
		return Command{}, newParseError(nil, "MW set: this record was read with P1=%q but slot %v derives P1=%q from its class; writing it would put one half of a section-defined channel into the other (590:1529-1531)", rec.AnswerP1, rec.Slot, want)
	}

	// An empty channel is never written. The only documented way to clear a
	// channel is the short MW of 590:1579-1581, which this milestone does not
	// build, and the TS-480 has no erase route at all — the one "clear" in
	// its whole book is RC, "Clears the RIT offset frequency" (480:1205).
	if rec.Empty {
		return Command{}, newParseError(nil, "MW set: this record is the empty channel of 590:1492-1493 (A18a), and an empty channel is not written — the only documented clear is the short MW of 590:1579-1581, which this milestone never builds (decision 8, A5)")
	}
	if rec.FreqHz == 0 {
		return Command{}, newParseError(nil, "MW set: P4 is zero, which is the empty channel's own value (590:1492-1493) rather than a frequency to write")
	}
	if rec.FreqHz > MaxRecordFreqHz {
		return Command{}, &OutOfDomainError{Field: "P4, the memory frequency", Value: rec.FreqHz, Digits: recFreqDigits, Max: MaxRecordFreqHz}
	}

	// A18b: a mode nibble of '0' or '8' is never legal on a build. Both books
	// call the two "None (setting failure)" or "Not used" (590:1353,
	// 590:1362; 480:843, 480:853) without saying what a Set carrying one
	// does, and the programme has no reason to send one — a codeplug channel
	// either has a mode or is empty, and an empty channel is not written.
	// The check is MEMBERSHIP of this layout's own legend, which excludes
	// both nibbles by construction (NewLayout refuses a legend naming
	// either).
	if _, ok := l.ParseMode(rec.Mode.Wire()); !ok {
		return Command{}, newParseError(nil, "MW set: P5 is %q, which the %s's MD legend does not name as a mode; nibbles '0' and '8' are a setting failure or unused in both books and what a Set carrying one does is unprinted (A18b)", rec.Mode.Wire(), l.model)
	}

	if rec.Byte19 != '0' && rec.Byte19 != '1' {
		return Command{}, newParseError(nil, "MW set: byte 19 is %q, and it is %v on the %s, whose book prints only '0' and '1' there", rec.Byte19, l.byte19, l.model)
	}
	if !l.ValidToneMode(rec.ToneMode) {
		return Command{}, newParseError(nil, "MW set: P7 is %q, and the %s prints %s", rec.ToneMode.Wire(), l.model, l.toneModeText())
	}
	// A21, and it is a REFUSAL rather than a clamp. The 590 pair print "An
	// entered value of 43 or higher results in an error" for TN (590:2309)
	// and nothing at all for CN; the 480 prints nothing for either, and what
	// the rule does inside an MW record is unprinted on all three rows.
	// Clamping would store a tone the user did not ask for and report
	// success.
	if rec.ToneIndex < MinToneIndex || rec.ToneIndex > MaxToneIndex {
		return Command{}, newParseError(nil, "MW set: P8 is %d, and TN prints %02d ~ %d (590:2291, 480:1557); an index outside its own chart is refused rather than clamped (A21)", rec.ToneIndex, MinToneIndex, MaxToneIndex)
	}
	// hasTail is whether this row's record reaches past P8 at all — see
	// parseRecordFrame's own hasTail for the citation. A row without a
	// tail never has these fields to validate or encode, and rec's own
	// zero values for them (a caller of an existing 50-byte driver never
	// sets DCSCode/Shift/OffsetHz, and CTCSSIndex/Byte28/Byte3940/Byte41
	// simply are not consulted here) are simply never reached.
	hasTail := l.recordLen == RecordLen
	if hasTail {
		if rec.CTCSSIndex < MinCTCSSIndex || rec.CTCSSIndex > MaxCTCSSIndex {
			return Command{}, newParseError(nil, "MW set: P9 is %d, and CN prints %02d ~ %d (590:411, 480:337); an index outside its own chart is refused rather than clamped (A21)", rec.CTCSSIndex, MinCTCSSIndex, MaxCTCSSIndex)
		}
		if err := l.checkP10(uint64(rec.DCSCode)); err != nil {
			return Command{}, newParseError(nil, "MW set: %v", err)
		}
		if err := l.checkByte28(rec.Byte28); err != nil {
			return Command{}, newParseError(nil, "MW set: %v", err)
		}
		if err := l.checkP12(rec.Shift); err != nil {
			return Command{}, newParseError(nil, "MW set: %v", err)
		}
		if err := l.checkP13(rec.OffsetHz); err != nil {
			return Command{}, newParseError(nil, "MW set: %v", err)
		}
		if err := l.checkByte3940(rec.Byte3940Wire()); err != nil {
			return Command{}, newParseError(nil, "MW set: %v", err)
		}
		if err := l.checkByte41(rec.Byte41); err != nil {
			return Command{}, newParseError(nil, "MW set: %v", err)
		}
		if len(rec.Name) > recNameLen {
			return Command{}, newParseError([]byte(rec.Name), "MW set: the memory name is %d bytes and P16 holds %d (590:1576, 480:984)", len(rec.Name), recNameLen)
		}
		for i := 0; i < len(rec.Name); i++ {
			if err := checkNameByte(rec.Name[i]); err != nil {
				return Command{}, newParseError([]byte(rec.Name), "MW set: memory name byte %d: %v (A2)", i+1, err)
			}
		}
	}

	wire, err := l.slotWire(rec.Slot)
	if err != nil {
		return Command{}, err
	}

	frame := make([]byte, l.recordLen)
	frame[recPrefixOff], frame[recPrefixOff+1] = 'M', 'W'
	frame[recP1Off] = rec.Slot.P1()
	copy(frame[recP2Off:], wire)
	copy(frame[recFreqOff:], fmt.Sprintf("%0*d", recFreqDigits, rec.FreqHz))
	frame[recModeOff] = rec.Mode.Wire()
	frame[recByte19Off] = rec.Byte19
	frame[recToneModeOff] = rec.ToneMode.Wire()
	copy(frame[recToneOff:], fmt.Sprintf("%0*d", recToneDigits, rec.ToneIndex))
	if hasTail {
		copy(frame[recCTCSSOff:], fmt.Sprintf("%0*d", recCTCSSDigits, rec.CTCSSIndex))
		copy(frame[recDCSOff:], fmt.Sprintf("%0*d", recDCSDigits, rec.DCSCode))
		frame[recByte28Off] = rec.Byte28
		frame[recShiftOff] = rec.Shift
		copy(frame[recOffsetOff:], fmt.Sprintf("%0*d", recOffsetDigits, rec.OffsetHz))
		copy(frame[recByte3940Off:], rec.Byte3940Wire())
		frame[recByte41Off] = rec.Byte41
		// A1, cited at the site: 8 bytes, padded with SPACES on write and
		// right-trimmed on read. Neither book states the rule for P16; the
		// 480's KY gives the same-document precedent for a different
		// command (480:785-787), and the lift is a write-then-read on each
		// registry row.
		copy(frame[recNameOff:], fmt.Sprintf("%-*s", recNameLen, rec.Name))
	} else {
		// The TS-570's own "P9: NOT USED" span — positions 23-27, five
		// bytes this row's own document assigns no meaning to at all
		// (evidence/ts570d-transcription.csv: "unused_filler"; the MR/MW
		// diagrams print no field there, only the terminator at position
		// 28). A frame that left them Go's zero byte failed the outbound
		// envelope's printable-ASCII rule (envelopeAllows, framing.go) and
		// validMWCommand's own rebuild-and-compare gate (allowlist.go),
		// so no 28-byte MW frame existed at all until this filled them.
		// '0' is the SAME filler P2Unused already writes one byte to the
		// west (slotWire) — ASSUMED rather than printed, since no book
		// says what a Set should carry where none prints a meaning, and
		// '0' is this family's own established choice for such a byte.
		for i := recToneOff + recToneDigits; i < int(l.recordLen)-1; i++ {
			frame[i] = '0'
		}
	}
	frame[l.recordLen-1] = ';'

	// THE PRINTED-FIXED BYTES ARE WRITTEN FROM THE LAYOUT'S OWN SET, not
	// from the record, and last, so no field above can overwrite one. The
	// 590 pair hard-wire thirteen of the 47 parameter bytes and the 480
	// sixteen; the three the two books share are P10, P12 and P13.
	for _, ff := range l.printedFixed {
		copy(frame[ff.Pos-1:], ff.Printed)
	}

	// The gate, through the same predicate the answer parser uses. IT IS AN
	// ASSERTION AND NOT THE MECHANISM: what actually keeps the short erase
	// form of 590:1579-1581 out is the fixed-length allocation above — no
	// input can change len(frame) — and this gate records that invariant so
	// an edit making the width variable would meet it. record_test.go is
	// explicit that no input can make it fire.
	if err := l.checkRecordLen("MW", len(frame), frame); err != nil {
		return Command{}, err
	}
	return newCommand(frame), nil
}

// Byte3940Wire returns rec's P14 bytes, and "\x00\x00" for a record whose
// field was never set — which is what makes an unset P14 a refusal at
// checkByte3940 rather than a silent "00".
//
// NO BYTE IS EMITTED WITHOUT A SOURCE. Bytes 39-40 are the FM Normal/Narrow
// flag on the 590 pair and the tuning step index on the 480, and neither
// book says what value means "no change"; writing "00" for a caller that
// said nothing would be inventing the one byte A22 and A23 exist because
// nobody can supply.
func (rec Record) Byte3940Wire() string {
	if len(rec.Byte3940) != recByte3940Len {
		return "\x00\x00"
	}
	return rec.Byte3940
}

// checkSlot re-resolves s against THIS layout's slot space.
//
// AGAINST THE RECEIVER, ALWAYS. A Slot is a value and may have been minted
// under another layout — a TS-590SG extension channel handed to a TS-480
// builder, or a slot above the ceiling A12 leaves the TS-590S. Trusting the
// class the value carries would let a frame legal only on another radio out
// of this builder.
func (l Layout) checkSlot(what string, s Slot) error {
	if s.IsZero() {
		return newParseError(nil, "%s: the slot was never resolved against a layout's slot space", what)
	}
	class := l.classOf(s.number)
	if class == SlotClassInvalid {
		return newParseError(nil, "%s: slot %d is outside the %s's slot space %s", what, s.number, l.model, l.slotSpaceText())
	}
	if class != s.class {
		return newParseError(nil, "%s: slot %d is %v on the %s, and this slot value carries %v — it was resolved against another layout", what, s.number, class, l.model, s.class)
	}
	if class == SlotScan && s.half != ScanLower && s.half != ScanUpper {
		return newParseError(nil, "%s: slot %d is a section-defined channel on the %s and the record does not say which of its two frequencies it carries (590:1529-1531)", what, s.number, l.model)
	}
	if class != SlotScan && s.half != ScanHalfNone {
		return newParseError(nil, "%s: slot %d is %v on the %s, which holds one frequency, so it has no half to name", what, s.number, class, l.model)
	}
	return nil
}

// slotWire renders s's P2 and P3 bytes under this layout's byte-4 policy.
//
// IT EMITS '0' RATHER THAN A SPACE below 100 on the 590 pair, WHICH IS A10.
// MC prints both as legal on a Set — "enter 0 or a space for a channel number
// less than 100" (590:1334-1335) — but MR and MW themselves only say "refer
// to the MC command" (590:1452-1453 for the MR answer, 590:1539-1540 for the
// MW Set), so that the same convention governs THIS pair of frames is
// assumed rather than printed. A digit is the one of the two that also passes
// the outbound envelope unremarkably and reads the same in a log. A10's lift
// is the dangerous half: a P2 the radio reads differently on a Set writes a
// channel the operator did not name. The ANSWER's space is a different
// direction and is what parseSlot admits.
//
// IT RETURNS AN ERROR BECAUSE ITS DEFAULT MUST REFUSE. This is the last site
// in the package to read a layout axis, and a zero Layout has no byte-4
// policy: emitting three digits there would be a permissive default on an
// unset axis, which is the FT-891 Stage 0 failure and the rule layout.go
// states in capitals. Both callers already return errors.
//
// The ceiling arm is an ASSERTION, not the mechanism. NewLayout refuses
// P2FixedZero alongside any slot above maxFixedZeroSlot (validateSlots), so
// no layout this package mints can reach it; a Slot is a value that may have
// been minted anywhere, and the truncation this replaces rendered channel
// 103 as "003" and returned success — a frame naming a DIFFERENT channel,
// reported as Sent. TestSlotWire_HasNoPermissiveDefaultAndNoSilentTruncation
// pins all three arms.
func (l Layout) slotWire(s Slot) (string, error) {
	switch l.p2 {
	case P2FixedZero:
		if s.number > maxFixedZeroSlot {
			return "", newParseError(nil, "slot %d cannot be named on the %s: byte 4 prints \"Always 0\" there (480:953) and P3 holds \"00 ~ 99\" (480:955), so there is no digit to carry the hundreds", s.number, l.model)
		}
		return fmt.Sprintf("0%02d", s.number), nil
	case P2Unused:
		// The book prints nothing for this byte at all (P2Unused's own
		// doc comment), so no wire value is asserted; '0' is written as
		// the filler every other byte in this family's empty/unused
		// positions uses, ASSUMED rather than printed.
		if s.number > maxFixedZeroSlot {
			return "", newParseError(nil, "slot %d cannot be named on the %s: byte 4 carries no hundreds digit (the %s's Parameter Table prints no meaning for it) and P3 holds two digits, so there is no digit to carry the hundreds", s.number, l.model, l.model)
		}
		return fmt.Sprintf("0%02d", s.number), nil
	case P2HundredsDigit:
		return fmt.Sprintf("%03d", s.number), nil
	default:
		return "", newParseError(nil, "byte 4's policy is unset on this layout — refusing to guess whether it is the channel's hundreds digit or a printed constant")
	}
}
