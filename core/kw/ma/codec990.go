// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import "strings"

// THE TS-990S MA0 CODEC, HAND-WRITTEN AGAINST THAT BOOK'S OWN POSITION RULER.
//
// Set and Answer share one grid (990:2893-2915 Set, 990:2919-2938 Answer; the
// parameter list 990:2891-2965), EIGHTEEN parameters over a FIXED 57 bytes:
// the ruler runs to 57 and nails ';' there, and the name sits in a fixed
// ten-byte window at positions 47-56. Nothing floats, so this row is decoded
// by fixed offsets throughout — the opposite half of spec decision 10.
//
// IT IS NOT THE TS-890S GRID WITH FIVE MORE FIELDS. This radio spends
// position 7 on a channel-type byte the other has no equivalent for, gives
// "frequency 2" a complete tone tuple of its own, adds a dual-reception flag,
// and encodes scan lockout 1/2 where the other encodes 0/1 — so no field of
// one sits at a shifted offset in the other, which is why there is no
// geometry table (plan P24).
const (
	// ma990Len is the printed frame length, fixed (990:2915, 990:2938).
	ma990Len = 57

	// The eighteen fields, as 0-based offsets. Positions 1-3 are the opcode
	// and 4-6 the channel number, both in shared.go.
	ma990ClassOff      = 6  // P2, position 7 (990:2897-2903)
	ma990FreqOff       = 7  // P3, positions 8-18 (990:2905-2906)
	ma990ModeOff       = 18 // P4, position 19 (990:2907-2910)
	ma990NarrowOff     = 19 // P5, position 20 (990:2912-2914)
	ma990ToneTypeOff   = 20 // P6, position 21 (990:2915-2919)
	ma990ToneOff       = 21 // P7, positions 22-23 (990:2920-2922)
	ma990CTCSSOff      = 23 // P8, positions 24-25 (990:2924-2926)
	ma990TXFreqOff     = 25 // P9, positions 26-36 (990:2927-2928)
	ma990TXModeOff     = 36 // P10, position 37 (990:2929-2931)
	ma990TXNarrowOff   = 37 // P11, position 38 (990:2932-2934)
	ma990TXToneTypeOff = 38 // P12, position 39 (990:2935-2939)
	ma990TXToneOff     = 39 // P13, positions 40-41 (990:2940-2942)
	ma990TXCTCSSOff    = 41 // P14, positions 42-43 (990:2943-2945)
	ma990SplitOff      = 43 // P15, position 44 (990:2946-2948)
	ma990DualOff       = 44 // P16, position 45 (990:2949-2951)
	ma990LockoutOff    = 45 // P17, position 46 (990:2952-2954)
	ma990NameOff       = 46 // P18, positions 47-56 (990:2955-2956)

	// ma990EmptyLo and ma990EmptyHi bound the blank-channel predicate:
	// "When reading a blank channel, parameters P2 to P18 become blank"
	// (990:2962-2963). Unlike the TS-890S's, this note INCLUDES the name,
	// which is why this row has no name-residue arm.
	ma990EmptyLo = ma990ClassOff
	ma990EmptyHi = ma990NameOff + ma0MaxNameLen - 1

	// ma990SecondLo and ma990SecondHi bound the frequency-2 side, P9-P14:
	// "When reading a single memory channel, all parameters for frequency 2
	// become 0" (990:2964-2965) — A16, DOCUMENTED. P16 is NOT in it: dual
	// reception is its own flag and the note does not name it.
	ma990SecondLo = ma990TXFreqOff
	ma990SecondHi = ma990TXCTCSSOff + ma0ToneDigits - 1

	// ma990LockoutOffByte and ma990LockoutOnByte are erratum E8: this
	// radio prints "1: Scan Lockout OFF / 2: Scan Lockout ON"
	// (990:2952-2954) where the TS-890S prints 0/1 and where this book's own
	// MA3 P2 prints "0: OFF / 1: ON". A codec that accepted '0' here would
	// be importing MA3's convention into MA0's field.
	ma990LockoutOffByte = '1'
	ma990LockoutOnByte  = '2'
)

// parseMA0Answer990 decodes one TS-990S MA0 answer (990:2919-2938).
//
// THE BLANK-WINDOW PREDICATE RUNS BEFORE ANY PER-FIELD DOMAIN PARSE, AND ON
// THIS ROW THE CONSEQUENCE IS PRINTED RATHER THAN DERIVED: P17 is valid only
// as '1' or '2' (990:2952-2954) while a blank channel returns spaces across
// P2-P18 (990:2962-2963), so a codec that parsed fields first would raise a
// ParseError on an unused slot — and clone.ReadAll returns on the FIRST
// channel error (core/clone/read.go:65-67), so one unused slot would make the
// radio unreadable end to end.
// TestParseMA0Answer990_TheBlankWindowIsTestedBeforeAnyFieldDomain is the
// pin, and its red proof is a MUTATION: move this block below the class check
// and it fails.
func (l Layout) parseMA0Answer990(frame []byte) (Record, error) {
	const what = "MA0 answer"
	if len(frame) != ma990Len {
		return Record{}, newParseError(frame, "%s: the frame is %d bytes, and the TS-990S grid is exactly 57 — eighteen parameters, a ten-byte name window at 47-56 and ';' nailed to 57 (990:2919-2938)", what, len(frame))
	}
	if string(frame[:len(ma0Prefix)]) != ma0Prefix {
		return Record{}, newParseError(frame, "%s: missing %q prefix, got %q", what, ma0Prefix, frame[:len(ma0Prefix)])
	}
	if frame[ma990Len-1] != ';' {
		return Record{}, newParseError(frame, "%s: missing ';' terminator at position %d (990:2938)", what, ma990Len)
	}

	slot, err := l.parseSlotField(frame)
	if err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec := Record{Slot: slot}

	if isBlankWindow(frame[ma990EmptyLo : ma990EmptyHi+1]) {
		rec.Empty = true
		return rec, nil
	}

	// P2 IS READ AND KEPT, and it is the only place this codec learns which
	// of the three channel types the radio reported (990:2897-2903). It is
	// a parser output: the Set direction emits '0' whatever this says (A14).
	if b := frame[ma990ClassOff]; b < '0' || b > '2' {
		return Record{}, newParseError(frame, "%s: P2, the channel type, is %q, and the book prints three values — 0 Single, 1 Dual, 2 Section defined (990:2897-2903)", what, b)
	}
	rec.Class = frame[ma990ClassOff]

	if rec.FreqHz, err = decodeDigits("P3, frequency 1", frame[ma990FreqOff:ma990FreqOff+ma0FreqDigits]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if err = l.checkModeByte("P4, the mode for frequency 1", frame[ma990ModeOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec.Mode = frame[ma990ModeOff]
	if rec.FMNarrow, err = decodeFlag("P5, FM wide/narrow for frequency 1", frame[ma990NarrowOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if err = checkToneType("P6, the FM tone function for frequency 1", frame[ma990ToneTypeOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec.ToneType = frame[ma990ToneTypeOff]
	if rec.ToneIndex, err = parseToneIndex("P7, the tone frequency for frequency 1", frame[ma990ToneOff:ma990ToneOff+ma0ToneDigits], l.checkToneIndex); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if rec.CTCSSIndex, err = parseToneIndex("P8, the CTCSS frequency for frequency 1", frame[ma990CTCSSOff:ma990CTCSSOff+ma0ToneDigits], l.checkCTCSSIndex); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}

	// THE FREQUENCY-2 SIDE IS DOMAIN-CHECKED ONLY WHEN IT CARRIES CONTENT
	// (A16). A single memory channel answers with P9-P14 all zero
	// (990:2964-2965), and P10 = '0' is a mode byte this legend prints
	// "Unused" (990:3707) — so checking it unconditionally would refuse
	// every single-frequency channel on the radio.
	if !allBytes(frame[ma990SecondLo:ma990SecondHi+1], '0') {
		if rec.TXFreqHz, err = decodeDigits("P9, frequency 2", frame[ma990TXFreqOff:ma990TXFreqOff+ma0FreqDigits]); err != nil {
			return Record{}, newParseError(frame, "%s: %v", what, err)
		}
		if err = l.checkModeByte("P10, the mode for frequency 2", frame[ma990TXModeOff]); err != nil {
			return Record{}, newParseError(frame, "%s: %v — a frame whose P9-P14 are not all zero is not the single memory channel of 990:2964-2965 (A16). P10 reads the OM command's P1 by this chart's own wording, which is erratum E9; the legend it means is OM P2's", what, err)
		}
		rec.TXMode = frame[ma990TXModeOff]
		if rec.TXFMNarrow, err = decodeFlag("P11, FM wide/narrow for frequency 2", frame[ma990TXNarrowOff]); err != nil {
			return Record{}, newParseError(frame, "%s: %v", what, err)
		}
		if err = checkToneType("P12, the FM tone function for frequency 2", frame[ma990TXToneTypeOff]); err != nil {
			return Record{}, newParseError(frame, "%s: %v", what, err)
		}
		rec.TXToneType = frame[ma990TXToneTypeOff]
		if rec.TXToneIndex, err = parseToneIndex("P13, the tone frequency for frequency 2", frame[ma990TXToneOff:ma990TXToneOff+ma0ToneDigits], l.checkToneIndex); err != nil {
			return Record{}, newParseError(frame, "%s: %v", what, err)
		}
		if rec.TXCTCSSIndex, err = parseToneIndex("P14, the CTCSS frequency for frequency 2", frame[ma990TXCTCSSOff:ma990TXCTCSSOff+ma0ToneDigits], l.checkCTCSSIndex); err != nil {
			return Record{}, newParseError(frame, "%s: %v", what, err)
		}
	}

	if rec.Split, err = decodeFlag("P15, the split information", frame[ma990SplitOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if rec.DualRecv, err = decodeFlag("P16, dual reception", frame[ma990DualOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	switch frame[ma990LockoutOff] {
	case ma990LockoutOffByte:
		rec.Lockout = false
	case ma990LockoutOnByte:
		rec.Lockout = true
	default:
		return Record{}, newParseError(frame, "%s: P17, the scan lockout, is %q, and this radio prints \"1: Scan Lockout OFF / 2: Scan Lockout ON\" (990:2952-2954) — erratum E8, and accepting '0' here would import MA3's 0/1 convention into MA0's field", what, frame[ma990LockoutOff])
	}
	if rec.Name, err = parseName("P18, the channel name", frame[ma990NameOff:ma990NameOff+ma0MaxNameLen]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	return rec, nil
}

// buildMA0Set990 builds the TS-990S memory write for rec (990:2893-2915).
//
// P2 IS EMITTED AS '0' AND IT IS THE MILESTONE'S ONLY DEFAULTED BYTE — A14.
// The book says the channel type "is decided while setting the P9 and P10
// values, so this parameter is ignored. Enter a dummy value" (990:2901-2903)
// and names no value, so this codec chooses one, records the choice in the
// ASSUMED register and publishes its lift (L-HW-11: Set with P2 = '0' and
// again with P2 = '9', reading back each time). Every other byte this
// milestone writes has a source in the record or is a refusal — decision 7.
//
// THE NAME IS PADDED TO ITS FIXED TEN-BYTE WINDOW WITH ASCII SPACE, WHICH IS
// A1. That book prints "Up to 10 digits" against a fixed window
// (990:2955-2956) — "up to" and a fixed window cannot both be literal — and
// states no pad byte (erratum E13); the strongest in-book support is KY's
// "Characters that are left blank will be filled with spaces"
// (990:2788-2789), which is about the CW keying buffer. The TS-890S half is
// not an assumption and is not here: its terminator floats and its encoder
// pads nothing.
func (l Layout) buildMA0Set990(rec Record) (Command, error) {
	const what = "MA0 set"
	if err := l.checkRecordCommon(rec); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := checkFreq("P3, frequency 1", rec.FreqHz); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkModeByte("P4, the mode for frequency 1", rec.Mode); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := checkToneType("P6, the FM tone function for frequency 1", rec.ToneType); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkToneIndex("P7, the tone frequency for frequency 1", rec.ToneIndex); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkCTCSSIndex("P8, the CTCSS frequency for frequency 1", rec.CTCSSIndex); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := checkName("P18, the channel name", rec.Name); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}

	second, err := l.buildSecond990(what, rec)
	if err != nil {
		return Command{}, err
	}

	lockout := byte(ma990LockoutOffByte)
	if rec.Lockout {
		lockout = ma990LockoutOnByte
	}

	var b strings.Builder
	b.WriteString(ma0Prefix)
	b.WriteString(rec.Slot.String())
	b.WriteByte('0') // P2 — A14, and the only defaulted byte in the milestone
	b.WriteString(encodeFreq(rec.FreqHz))
	b.WriteByte(rec.Mode)
	b.WriteByte(flagByte(rec.FMNarrow))
	b.WriteByte(rec.ToneType)
	b.WriteString(encodeToneIndex(rec.ToneIndex))
	b.WriteString(encodeToneIndex(rec.CTCSSIndex))
	b.WriteString(second)
	b.WriteByte(flagByte(rec.Split))
	b.WriteByte(flagByte(rec.DualRecv))
	b.WriteByte(lockout)
	b.WriteString(rec.Name)
	b.WriteString(strings.Repeat(" ", ma0MaxNameLen-len(rec.Name)))
	b.WriteByte(';')

	frame := []byte(b.String())
	if len(frame) != ma990Len {
		return Command{}, newParseError(frame, "%s: built %d bytes, want exactly %d (990:2915)", what, len(frame), ma990Len)
	}
	return newCommand(frame), nil
}

// buildSecond990 renders P9-P14, the frequency-2 side.
//
// A record with no second mode emits the printed ZEROED form (990:2964-2965,
// A16), which is what makes an ordinary single-frequency channel round-trip
// with no refusal at all; a record claiming that while carrying frequency-2
// content is refused rather than half emitted.
func (l Layout) buildSecond990(what string, rec Record) (string, error) {
	if rec.TXMode == 0 {
		if rec.TXFreqHz != 0 || rec.TXFMNarrow || rec.TXToneType != 0 || rec.TXToneIndex != 0 || rec.TXCTCSSIndex != 0 || rec.Split || rec.DualRecv {
			return "", newParseError(nil, "%s: this record has no second side mode but carries frequency-2 content (P9 = %d, P11 = %v, P12 = %q, P13 = %d, P14 = %d, P15 = %v, P16 = %v); a single memory channel's whole frequency-2 side is zero, which a split P15 or a dual-reception P16 contradicts (990:2946-2951, 990:2964-2965)", what, rec.TXFreqHz, rec.TXFMNarrow, rec.TXToneType, rec.TXToneIndex, rec.TXCTCSSIndex, rec.Split, rec.DualRecv)
		}
		return strings.Repeat("0", ma990SecondHi-ma990SecondLo+1), nil
	}
	if err := checkFreq("P9, frequency 2", rec.TXFreqHz); err != nil {
		return "", newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkModeByte("P10, the mode for frequency 2", rec.TXMode); err != nil {
		return "", newParseError(nil, "%s: %v", what, err)
	}
	if err := checkToneType("P12, the FM tone function for frequency 2", rec.TXToneType); err != nil {
		return "", newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkToneIndex("P13, the tone frequency for frequency 2", rec.TXToneIndex); err != nil {
		return "", newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkCTCSSIndex("P14, the CTCSS frequency for frequency 2", rec.TXCTCSSIndex); err != nil {
		return "", newParseError(nil, "%s: %v", what, err)
	}
	return encodeFreq(rec.TXFreqHz) +
		string(rec.TXMode) +
		string(flagByte(rec.TXFMNarrow)) +
		string(rec.TXToneType) +
		encodeToneIndex(rec.TXToneIndex) +
		encodeToneIndex(rec.TXCTCSSIndex), nil
}
