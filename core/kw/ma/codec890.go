// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"bytes"
	"strings"
)

// THE TS-890S MA0 CODEC, HAND-WRITTEN AGAINST THAT BOOK'S OWN POSITION RULER.
//
// Set and Answer share one grid (890:3166-3182 Set, 890:3187-3204 Answer;
// the parameter list 890:3164-3221), thirteen parameters over thirty-nine
// fixed positions followed by a name and a FLOATING TERMINATOR: the ruler's
// last header cell is the letter "x", never a number, and ';' therefore sits
// at 40 + len(name). That is the whole reason this row is parsed by scanning
// for the terminator rather than by fixed offsets, and the reason its round
// trip is pinned at THREE name lengths — a fixed-offset bug is invisible at
// one.
//
// EVERY POSITION THIS FILE FILLS IS ONE THE BOOK PRINTS AS A HOST-TO-RADIO
// FORM, with its grid line cited at the constant that locates it.
const (
	// ma890MinLen is the zero-character name's frame, which is A17: DERIVED
	// from "Up to 10 characters" (890:3208-3209) plus the floating "x"
	// (890:3181-3182), and printed as a worked frame nowhere in the book.
	ma890MinLen = 40
	// ma890MaxLen is byte 49 for the tenth name character plus the
	// terminator at 50.
	ma890MaxLen = ma890MinLen + ma0MaxNameLen

	// The thirty-nine fixed positions, as 0-based offsets. Positions 1-3
	// are the opcode and 4-6 the channel number, both in shared.go.
	ma890FreqOff     = 6  // P2, positions 7-17 (890:3171-3172)
	ma890ModeOff     = 17 // P3, position 18 (890:3174-3175)
	ma890NarrowOff   = 18 // P4, position 19 (890:3176-3178)
	ma890ToneTypeOff = 19 // P5, position 20 (890:3180-3185)
	ma890ToneOff     = 20 // P6, positions 21-22 (890:3186-3187)
	ma890CTCSSOff    = 22 // P7, positions 23-24 (890:3188-3190)
	ma890TXFreqOff   = 24 // P8, positions 25-35 (890:3191-3192)
	ma890TXModeOff   = 35 // P9, position 36 (890:3193-3195)
	ma890TXNarrowOff = 36 // P10, position 37 (890:3197-3200)
	ma890SplitOff    = 37 // P11, position 38 (890:3201-3203)
	ma890LockoutOff  = 38 // P12, position 39 (890:3205-3207)
	ma890NameOff     = 39 // P13, position 40 onwards (890:3208-3209)

	// ma890EmptyLo and ma890EmptyHi bound the blank-channel predicate's
	// window: "When reading a blank channel, parameters P2 to P12 becomes
	// blank" (890:3215-3216). It STOPS AT P12 — erratum E4 — so P13 is
	// outside it, and so is the frame's LENGTH (A21).
	ma890EmptyLo = ma890FreqOff
	ma890EmptyHi = ma890LockoutOff

	// ma890SecondLo and ma890SecondHi bound the split-transmission side,
	// P8-P10: "When reading a single memory channel, all parameters for
	// Split Transmission become 0" (890:3217-3218) — A16, DOCUMENTED.
	ma890SecondLo = ma890TXFreqOff
	ma890SecondHi = ma890TXNarrowOff
)

// ma0AnswerMatcher890 is this row's answer matcher: the prefix comparison
// MA0AnswerMatcher describes, with a length RANGE rather than an equality.
//
// PrefixLenMatcher's exactLen <= 0 BRANCH IS DELIBERATELY NOT USED, and
// core/kw's own doc.go is why: it records that a second variable-length user
// of that branch "is a sign that somebody has mis-read a chart", and the same
// file already names this radio — "The TS-890S's floating MA0 terminator is
// not in this pair." A bare unbounded matcher would also correlate a
// two-hundred-byte run of noise that happened to start with the right six
// bytes. A range matcher correlates what the book can produce and refuses
// what it cannot, and still delivers a corrupt-but-plausible frame to the
// parser, so such a frame is refused with a message rather than reported as a
// timeout — which is pair 1's stated principle for MRAnswerMatcher, honoured
// here with eight lines instead of a slot decoder.
func ma0AnswerMatcher890(prefix string) func(frame []byte) bool {
	return func(frame []byte) bool {
		if len(frame) < ma890MinLen || len(frame) > ma890MaxLen {
			return false
		}
		return string(frame[:len(prefix)]) == prefix
	}
}

// parseMA0Answer890 decodes one TS-890S MA0 answer (890:3187-3204).
//
// THE STRUCTURAL CHECKS COME IN core/kw'S FIXED ORDER — length, prefix,
// terminator — so that a frame wrong in several ways reports the same failure
// whichever answer it was offered to.
//
// THEN THE BLANK-WINDOW PREDICATE, BEFORE ANY PER-FIELD DOMAIN PARSE, AND
// THAT ORDER IS NORMATIVE. A blank channel's P3 is blank, and neither ' ' nor
// '0' is a mode either legend names, so a codec that parsed fields first
// would raise a ParseError on an unused slot — and clone.ReadAll returns on
// the FIRST channel error (core/clone/read.go:65-67), so ONE unused slot
// would make the radio unreadable end to end.
// TestParseMA0Answer890_TheBlankWindowIsTestedBeforeAnyFieldDomain is the
// pin, and its red proof is a MUTATION: move this block below the mode check
// and it fails.
func (l Layout) parseMA0Answer890(frame []byte) (Record, error) {
	const what = "MA0 answer"
	if len(frame) < ma890MinLen || len(frame) > ma890MaxLen {
		return Record{}, newParseError(frame, "%s: the frame is %d bytes, and the TS-890S grid runs 40 to 50 — thirty-nine printed positions, a name of up to ten characters (890:3208-3209) and a terminator that floats at 40 + len(name) (890:3181-3182); the 40-byte minimum is A17", what, len(frame))
	}
	if string(frame[:len(ma0Prefix)]) != ma0Prefix {
		return Record{}, newParseError(frame, "%s: missing %q prefix, got %q", what, ma0Prefix, frame[:len(ma0Prefix)])
	}
	// THE TERMINATOR IS SCANNED FOR, NOT INDEXED. It floats, so its
	// position is a reading rather than a constant; and requiring the FIRST
	// ';' to be the last byte is what makes "one frame" mean the same thing
	// here as it does to the radio's own parser.
	if i := bytes.IndexByte(frame, ';'); i != len(frame)-1 {
		return Record{}, newParseError(frame, "%s: the ';' terminator is not the last byte — the ruler heads it \"x\" and it floats at 40 + len(name) (890:3181-3182), so a frame carrying an earlier ';' is two frames to the radio's own parser", what)
	}

	slot, err := l.parseSlotField(frame)
	if err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec := Record{Slot: slot}

	if isBlankWindow(frame[ma890EmptyLo : ma890EmptyHi+1]) {
		rec.Empty = true
		// A21: the note stops at P12, so a fresh radio may answer a blank
		// channel with a residue in its floating name window. The residue
		// is CARRIED rather than discarded or raised on — reporting the
		// channel unassigned is the safe reading, and an error here would
		// abandon the whole radio's read.
		rec.NameResidue = strings.TrimRight(string(frame[ma890NameOff:len(frame)-1]), " ")
		return rec, nil
	}

	if rec.FreqHz, err = decodeDigits("P2, the frequency", frame[ma890FreqOff:ma890FreqOff+ma0FreqDigits]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if err = l.checkModeByte("P3, the mode", frame[ma890ModeOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec.Mode = frame[ma890ModeOff]
	if rec.FMNarrow, err = decodeFlag("P4, the FM normal/narrow information", frame[ma890NarrowOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if err = checkToneType("P5, the FM tone type", frame[ma890ToneTypeOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec.ToneType = frame[ma890ToneTypeOff]
	if rec.ToneIndex, err = parseToneIndex("P6, the tone frequency", frame[ma890ToneOff:ma890ToneOff+ma0ToneDigits], l.checkToneIndex); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if rec.CTCSSIndex, err = parseToneIndex("P7, the CTCSS frequency", frame[ma890CTCSSOff:ma890CTCSSOff+ma0ToneDigits], l.checkCTCSSIndex); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}

	// THE SPLIT SIDE IS DOMAIN-CHECKED ONLY WHEN IT CARRIES CONTENT (A16).
	// A single memory channel answers with P8-P10 all zero (890:3217-3218),
	// and P9 = '0' is a mode byte the legend prints "Unused" — so checking
	// it unconditionally would refuse every unsplit channel on the radio.
	if !allBytes(frame[ma890SecondLo:ma890SecondHi+1], '0') {
		if rec.TXFreqHz, err = decodeDigits("P8, the split transmission frequency", frame[ma890TXFreqOff:ma890TXFreqOff+ma0FreqDigits]); err != nil {
			return Record{}, newParseError(frame, "%s: %v", what, err)
		}
		if err = l.checkModeByte("P9, the split transmission mode", frame[ma890TXModeOff]); err != nil {
			return Record{}, newParseError(frame, "%s: %v — a frame whose P8-P10 are not all zero is not the single memory channel of 890:3217-3218 (A16)", what, err)
		}
		rec.TXMode = frame[ma890TXModeOff]
		if rec.TXFMNarrow, err = decodeFlag("P10, the split transmission FM normal/narrow information", frame[ma890TXNarrowOff]); err != nil {
			return Record{}, newParseError(frame, "%s: %v", what, err)
		}
	}

	if rec.Split, err = decodeFlag("P11, the split information", frame[ma890SplitOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if rec.Lockout, err = decodeFlag("P12, the scan lockout", frame[ma890LockoutOff]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if rec.Name, err = parseName("P13, the channel name", frame[ma890NameOff:len(frame)-1]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	return rec, nil
}

// parseToneIndex decodes a two-digit tone index and applies check, which is
// whichever of the two ceilings the caller's field is bounded by.
func parseToneIndex(field string, window []byte, check func(string, int) error) (int, error) {
	v, err := decodeDigits(field, window)
	if err != nil {
		return 0, err
	}
	if err := check(field, int(v)); err != nil {
		return 0, err
	}
	return int(v), nil
}

// buildMA0Set890 builds the TS-890S memory write for rec (890:3166-3182).
//
// IT PADS NOTHING. The name is emitted as-is and the terminator follows it,
// so the frame is 40 + len(name) bytes; the fixed ten-byte window is the
// TS-990S's shape, not this one's, and A1's pad rule is scoped to that row.
func (l Layout) buildMA0Set890(rec Record) (Command, error) {
	const what = "MA0 set"
	if err := l.checkRecordCommon(rec); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}

	// THE FIELDS THIS GRID DOES NOT HAVE ARE REFUSED, NOT DROPPED. The
	// TS-890S has ONE tone pair (890:3186-3190), no dual-reception flag and
	// no channel-type byte; a record carrying any of them was read from the
	// other radio, and emitting a frame that silently omitted them would be
	// data loss wearing a successful write.
	if rec.TXToneType != 0 || rec.TXToneIndex != 0 || rec.TXCTCSSIndex != 0 {
		return Command{}, newParseError(nil, "%s: this record carries a second tone tuple (P12-P14 on the TS-990S, 990:2935-2945) and the TS-890S grid has one tone pair, at P6-P7 (890:3186-3190) — there is no position to put it in", what)
	}
	if rec.DualRecv {
		return Command{}, newParseError(nil, "%s: this record carries dual reception (P16 on the TS-990S, 990:2949-2951) and the TS-890S grid has no such parameter", what)
	}
	if rec.Class != 0 {
		return Command{}, newParseError(nil, "%s: this record carries a channel type of %q (P2 on the TS-990S, 990:2897-2903) and the TS-890S grid has no channel type byte", what, rec.Class)
	}

	if err := checkFreq("P2, the frequency", rec.FreqHz); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkModeByte("P3, the mode", rec.Mode); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := checkToneType("P5, the FM tone type", rec.ToneType); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkToneIndex("P6, the tone frequency", rec.ToneIndex); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkCTCSSIndex("P7, the CTCSS frequency", rec.CTCSSIndex); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}
	if err := checkName("P13, the channel name", rec.Name); err != nil {
		return Command{}, newParseError(nil, "%s: %v", what, err)
	}

	second, err := l.buildSecond890(what, rec)
	if err != nil {
		return Command{}, err
	}

	var b strings.Builder
	b.WriteString(ma0Prefix)
	b.WriteString(rec.Slot.String())
	b.WriteString(encodeFreq(rec.FreqHz))
	b.WriteByte(rec.Mode)
	b.WriteByte(flagByte(rec.FMNarrow))
	b.WriteByte(rec.ToneType)
	b.WriteString(encodeToneIndex(rec.ToneIndex))
	b.WriteString(encodeToneIndex(rec.CTCSSIndex))
	b.WriteString(second)
	b.WriteByte(flagByte(rec.Split))
	b.WriteByte(flagByte(rec.Lockout))
	b.WriteString(rec.Name)
	b.WriteByte(';')

	frame := []byte(b.String())
	if want := ma890MinLen + len(rec.Name); len(frame) != want {
		return Command{}, newParseError(frame, "%s: built %d bytes, want %d = 40 + len(name) (890:3181-3182)", what, len(frame), want)
	}
	return newCommand(frame), nil
}

// buildSecond890 renders P8-P10, the split transmission side.
//
// THE P4/P10 AGREEMENT RULE IS ENFORCED HERE AND IT IS A CODEC INVARIANT
// RATHER THAN A REFUSAL RUNG. "When setting the split memory channel, set the
// same setting on the transmission side and the reception side for FM normal
// / narrow information (P4, P10)" (890:3219-3221). The refusal is worth
// keeping at this boundary — but it is UNREACHABLE FROM THE DRIVER, because
// codeplug.ChannelData carries one FM width, so a driver's builder sets both
// sides from that one value and the two bytes it emits can never disagree.
// The printed rule is enforced on the RADIO side instead, by the write
// ladder's rung 11, which compares the slot's CURRENT P10 against what this
// Set would emit.
// TestBuildMA0Set890_P4EqualsP10IsACodecInvariantTheDriverCannotReach says so
// in its name and writes its red proof against a hand-built Record, never a
// ChannelData.
func (l Layout) buildSecond890(what string, rec Record) (string, error) {
	if rec.TXMode == 0 {
		// The printed zeroed form (890:3217-3218, A16). A record claiming
		// it while carrying split content is refused rather than half
		// emitted.
		if rec.TXFreqHz != 0 || rec.TXFMNarrow {
			return "", newParseError(nil, "%s: this record has no second side mode but carries split transmission content (P8 = %d, P10 = %v); a single memory channel's whole split side is zero (890:3217-3218)", what, rec.TXFreqHz, rec.TXFMNarrow)
		}
		return strings.Repeat("0", ma0FreqDigits+2), nil
	}
	if err := checkFreq("P8, the split transmission frequency", rec.TXFreqHz); err != nil {
		return "", newParseError(nil, "%s: %v", what, err)
	}
	if err := l.checkModeByte("P9, the split transmission mode", rec.TXMode); err != nil {
		return "", newParseError(nil, "%s: %v", what, err)
	}
	if rec.TXFMNarrow != rec.FMNarrow {
		return "", newParseError(nil, "%s: the receive side's FM normal/narrow information is %v and the transmission side's is %v — the book requires \"the same setting on the transmission side and the reception side for FM normal / narrow information (P4, P10)\" when setting a split memory channel (890:3219-3221)", what, rec.FMNarrow, rec.TXFMNarrow)
	}
	return encodeFreq(rec.TXFreqHz) + string(rec.TXMode) + string(flagByte(rec.TXFMNarrow)), nil
}
