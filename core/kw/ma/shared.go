// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// WHAT THE TWO MA0 GRIDS GENUINELY SHARE BELOW THE ENVELOPE, AND NOTHING
// SPECULATIVE.
//
// The two grids differ in FIELD COUNT — thirteen parameters against eighteen
// — and in FIELD MEANING, and no field of one sits at a shifted offset in the
// other, so the geometry is NOT carried as a table (spec decision 10, plan
// P24): a two-row table no third radio can join is an interface with one
// implementation dressed as data. What IS shared is this file: the 3-digit
// slot encoding, the 11-digit frequency encoding, the 2-digit tone index
// encoding, the four-value tone type, the 0/1 FM narrow flag, the name
// field's charset and pad rule, the empty-window predicate's SHAPE, and the
// MA0 Read frame byte for byte. Every one of them is a small helper the two
// hand-written codecs consume; each takes the CALLER's P-number as a label,
// because the same datum is P2 on one radio and P3 on the other and a helper
// that spelled its own would name the wrong parameter for one of them.

const (
	// ma0Prefix is the opcode both grids share; the channel number follows
	// it IMMEDIATELY, at positions 4-6 on both (890:3166-3168,
	// 990:2893-2895), which is what makes the correlation key a prefix.
	ma0Prefix = "MA0"

	// ma0SlotOff and ma0SlotDigits locate that channel number.
	ma0SlotOff    = 3
	ma0SlotDigits = 3

	// ma0ReadLen is "M A 0 P1 P1 P1 ;" — seven bytes on BOTH radios
	// (890:3184-3186, 990:2916-2918).
	ma0ReadLen = 7

	// ma0FreqDigits is the printed width of every frequency field in either
	// grid: "11 digits in Hz" (890:3171-3172, 890:3191-3192; 990:2905-2906,
	// 990:2927-2928).
	ma0FreqDigits = 11
	// ma0MaxFreqHz is the widest value that width holds. IT IS A FIELD
	// WIDTH AND NOTHING MORE: neither book prints a tuning range for a
	// memory channel, which is A15, so nothing here says a radio will
	// accept 99,999,999,999 Hz.
	ma0MaxFreqHz = 99_999_999_999

	// ma0ToneDigits is the printed width of every tone and CTCSS index in
	// either grid, two digits on both (890:3186-3190, 990:2920-2926).
	ma0ToneDigits = 2

	// ma0MaxNameLen is "Up to 10 characters" (890:3208-3209) and "Up to 10
	// digits" (990:2955-2956) — the same ten on both, reached by a floating
	// terminator on one radio and a fixed window on the other.
	ma0MaxNameLen = 10
)

// Slot is one MA-family memory slot resolved against a layout's slot space:
// its number and the class that number falls in.
//
// THE ZERO VALUE IS INVALID AND CARRIES NO CLASS, which matters because
// channel 000 is a real slot on both radios: a Slot cannot be tested for
// emptiness by its number.
//
// IT IS NOT A kw.Slot AND IT HAS NO HALF. kw.Slot carries a ScanHalf because
// the 590's section-defined channels reach two frequencies through one slot
// number with different P1 bytes; on this family P1 IS the channel number,
// three digits at positions 4-6, and the second frequency has its own field
// in the same record. A half here would be a claim neither book makes. (And
// kw.Slot could not be reached anyway: its only minter is kw.Layout.NewSlot,
// and no ma.Layout is a kw.Layout — spec decision 1.)
type Slot struct {
	number int
	class  kw.SlotClass
}

// Number is the slot's channel number.
func (s Slot) Number() int { return s.number }

// Class is the class the constructing layout resolved the number to.
func (s Slot) Class() kw.SlotClass { return s.class }

// IsZero reports whether s was never constructed by a layout.
func (s Slot) IsZero() bool { return s.class == kw.SlotClassInvalid }

// String is the slot's three printed digits — BOTH the identifier a refusal
// spells and the wire field a frame carries, because this family has no half
// suffix to separate the two. Both books print P1 as three cells over "000 ~
// 119" (890:3166-3169, 990:2893-2896), and A5 records that the ANSWER
// direction spells it back the same way rather than space-padding as the
// 590's MC does. The dual role is safe because every builder re-measures the
// assembled frame against its printed length (codec890.go:262,
// codec990.go:251, shared.go:321), so a future suffix would fail loudly
// rather than ship a malformed P1.
func (s Slot) String() string {
	if s.IsZero() {
		return "<invalid slot>"
	}
	return fmt.Sprintf("%0*d", ma0SlotDigits, s.number)
}

// NewSlot resolves number against this layout's slot space.
//
// A zero Layout has no slot space and so admits no slot at all.
func (l Layout) NewSlot(number int) (Slot, error) {
	if !l.Configured() {
		return Slot{}, newParseError(nil, "slot %d: this layout is unconfigured and describes no radio's slot space", number)
	}
	class := l.classOf(number)
	if class == kw.SlotClassInvalid {
		return Slot{}, newParseError(nil, "slot %d is outside the %s's slot space %s", number, l.model, l.slotSpaceText())
	}
	return Slot{number: number, class: class}, nil
}

// classOf resolves number against this layout's ranges, or
// kw.SlotClassInvalid.
func (l Layout) classOf(number int) kw.SlotClass {
	for _, r := range l.slots {
		if number >= r.Lo && number <= r.Hi {
			return r.Class
		}
	}
	return kw.SlotClassInvalid
}

// slotSpaceText renders this layout's slot ranges for a refusal.
func (l Layout) slotSpaceText() string {
	out := make([]string, 0, len(l.slots))
	for _, r := range l.slots {
		out = append(out, fmt.Sprintf("%03d-%03d %v", r.Lo, r.Hi, r.Class))
	}
	if len(out) == 0 {
		return "(empty)"
	}
	return strings.Join(out, ", ")
}

// Record is the decoded content of one MA0 answer, and the input one MA0 Set
// is built from. ONE type serves both grids, with the fields the 890S does
// not have left at their zero values on that row and refused by its builder
// rather than silently dropped.
//
// THE SECONDARY SIDE IS CARRIED THOUGH THE NEUTRAL MODEL HAS NO HOME FOR IT
// (spec decision 9). codeplug.ChannelData has one mode, one FM width and one
// tone tuple; the 890S gives the transmit side its own mode and width
// (890:3193-3200) and the 990S gives "frequency 2" a complete tuple plus a
// dual-reception flag (990:2929-2951). Those values live here so that the
// write path can QUOTE what it is refusing — which is what makes decision 9's
// message name P-numbers and both values instead of saying "unsupported".
//
// TXMode == 0 MEANS THE PRINTED ZEROED SECONDARY SIDE, and that is the one
// piece of encoding this type carries. Both books print it: "When reading a
// single memory channel, all parameters for Split Transmission become 0"
// (890:3217-3218) and "all parameters for frequency 2 become 0"
// (990:2964-2965) — A16, which is DOCUMENTED rather than assumed. Since '0'
// is a mode byte BOTH legends print "Unused" (890:3977, 990:3707), a
// secondary side of zeros cannot be represented as a mode value, and a
// parser that read it as one would refuse every unsplit channel on the radio.
// So a zero TXMode says "this record's secondary side is the printed zeroed
// form"; the decoder sets it on recognising that window and the encoder
// re-emits the window whole. THE SENTINEL IS SAFE RATHER THAN FORCED — a
// HasSecond bool was available too — because no legal Record can carry
// TXMode == 0 with secondary content: both builders refuse exactly that
// pairing (buildSecond890, buildSecond990, and MED-1's Split/DualRecv
// extension of the same guard).
type Record struct {
	// Slot is the channel this record is for, resolved to its class.
	Slot Slot

	// FreqHz is the receive frequency: 890S P2 (890:3171-3172), 990S P3
	// (990:2905-2906).
	FreqHz uint64

	// Mode is the OM P2 legend byte: 890S P3 (890:3174-3175), 990S P4
	// (990:2907-2910). There is no MD command on either radio.
	Mode byte

	// FMNarrow is the FM Normal/Narrow flag: 890S P4 (890:3176-3178), 990S
	// P5 (990:2912-2914). Neither legend scopes it to a particular mode
	// value.
	FMNarrow bool

	// ToneType is the four-value FM tone selector as its wire byte, '0'
	// OFF, '1' Tone, '2' CTCSS, '3' Cross Tone: 890S P5 (890:3180-3185),
	// 990S P6 (990:2915-2919).
	ToneType byte

	// ToneIndex is the TN index and CTCSSIndex the CN index — INDEPENDENT
	// transmit and receive indices: 890S P6/P7 (890:3186-3190), 990S P7/P8
	// (990:2920-2926).
	ToneIndex  int
	CTCSSIndex int

	// TXFreqHz, TXMode and TXFMNarrow are the secondary side both grids
	// have: 890S P8/P9/P10 (890:3191-3200), 990S P9/P10/P11
	// (990:2927-2934). See the type comment for what TXMode == 0 means.
	TXFreqHz   uint64
	TXMode     byte
	TXFMNarrow bool

	// TXToneType, TXToneIndex and TXCTCSSIndex are the SECOND tone tuple,
	// which only the TS-990S has: P12, P13 and P14 (990:2935-2945). The
	// TS-890S grid has one tone pair and no position for these, so its
	// builder refuses a record carrying them rather than shortening it
	// silently.
	TXToneType   byte
	TXToneIndex  int
	TXCTCSSIndex int

	// Split is the split flag: 890S P11 (890:3201-3203), 990S P15
	// (990:2946-2948).
	Split bool

	// DualRecv is dual reception, which only the TS-990S has: P16
	// (990:2949-2951). It names no field in the neutral model at all, which
	// is why decision 9's refusal for it is read-dependent and lives in the
	// driver.
	DualRecv bool

	// Lockout is scan lockout, and the two radios ENCODE IT DIFFERENTLY:
	// 890S P12 is "0: Lockout OFF / 1: Lockout ON" (890:3205-3207) while
	// 990S P17 is "1: Scan Lockout OFF / 2: Scan Lockout ON"
	// (990:2952-2954) — erratum E8. The neutral field is the boolean; each
	// codec spells it its own radio's way.
	Lockout bool

	// Name is the channel name, right-trimmed of trailing spaces on read:
	// 890S P13 (890:3208-3209), 990S P18 (990:2955-2956).
	//
	// ON THE 890S A TRAILING SPACE CANNOT SURVIVE A ROUND TRIP, and that is
	// a stated capability limit rather than a defect: the terminator floats
	// after the name so nothing is padded, and a trailing space is
	// indistinguishable from the absence of one (A1's 890S half). On the
	// 990S the window is fixed and the pad IS assumed — A1.
	Name string

	// Class is the TS-990S's P2 channel type EXACTLY AS READ — '0' Single,
	// '1' Dual, '2' Section defined (990:2897-2903) — and it is a PARSER
	// OUTPUT rather than a builder input, on core/kw's AnswerP1 precedent.
	// The Set direction cannot use it: that book says the parameter "is
	// ignored. Enter a dummy value", so this codec emits '0' and records
	// A14, the milestone's only defaulted byte. It is zero on a TS-890S
	// record, whose grid has no such field, and on a record a caller built.
	Class byte

	// Empty reports that the frame satisfied its book's blank-channel
	// predicate. It is set by the parser only; no builder here writes an
	// empty channel, because that would be an erase (decision 15).
	Empty bool

	// NameResidue is the TS-890S's name window on a channel the predicate
	// reported EMPTY, and it exists because that book's blank-channel note
	// stops at P12 (890:3215-3216, erratum E4) where the 990S's covers
	// P2-P18. A21 is the assumption that such a residue is not channel
	// content; carrying it in the record rather than discarding it or
	// raising on it is what keeps clone.ReadAll — which abandons a whole
	// radio's read on the FIRST channel error — reading a fresh 890S.
	// Always empty on a TS-990S record.
	NameResidue string
}

// ParseMA0Answer decodes one MA0 answer against this row's own grid.
//
// IT IS A METHOD AND NOT A PACKAGE FUNCTION, which is spec decision 10: a
// function handed a frame could not know which grid it was looking at and
// would have to infer the book from the byte count — precisely the inference
// this design exists to avoid.
func (l Layout) ParseMA0Answer(frame []byte) (Record, error) {
	switch l.book {
	case kw.Book890:
		return l.parseMA0Answer890(frame)
	case kw.Book990:
		return l.parseMA0Answer990(frame)
	}
	return Record{}, newParseError(frame, "MA0 answer: this layout is unconfigured and describes no radio, so no byte of this frame has a meaning to read")
}

// BuildMA0Set builds the memory-channel write for rec on this row's own grid.
// A METHOD, for ParseMA0Answer's reason.
//
// EVERY FRAME IT CAN BUILD IS ONE ITS BOOK PRINTS AS A HOST-TO-RADIO FORM,
// with all its parameters: 890:3166-3182 and 990:2893-2915 are the two Set
// grids, and each codec's own doc comment cites the grid line for every
// position it fills.
func (l Layout) BuildMA0Set(rec Record) (Command, error) {
	switch l.book {
	case kw.Book890:
		return l.buildMA0Set890(rec)
	case kw.Book990:
		return l.buildMA0Set990(rec)
	}
	return Command{}, newParseError(nil, "MA0 set: this layout is unconfigured and describes no radio")
}

// BuildMA0Read builds the memory-channel read for s: "M A 0 P1 P1 P1 ;",
// seven bytes, IDENTICAL on both radios (890:3184-3186, 990:2916-2918).
//
// A18 IS CITED HERE BECAUSE THIS FRAME IS WHERE IT BITES, and it is the
// load-bearing read assumption of the whole milestone: an MA0 Read needs no
// preceding MN. Both Read forms carry their own channel number, so the
// command is stateless on its face — but MA1, MA7 and MI all act on "the
// channel appointed when using this command", so the family HAS state and
// MA0 Read's independence from it is nowhere printed. Decision 5's removal of
// MN from the outbound roster PROMOTES the assumption rather than withdrawing
// it: with no MN builder the driver could not select a channel even if it had
// to. If A18 is false, no read works at all. Its lift is hardware item 10
// (L-HW-12): from VFO mode, with no MN sent, read channel 005.
func (l Layout) BuildMA0Read(s Slot) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "MA0 read: this layout is unconfigured and describes no radio")
	}
	if err := l.checkSlot(s); err != nil {
		return Command{}, newParseError(nil, "MA0 read: %v", err)
	}
	frame := []byte(ma0Prefix + s.String() + ";")
	if len(frame) != ma0ReadLen {
		return Command{}, newParseError(frame, "MA0 read: built %d bytes, want exactly %d (890:3184-3186, 990:2916-2918)", len(frame), ma0ReadLen)
	}
	return newCommand(frame), nil
}

// MA0AnswerMatcher returns the answer matcher for one MA0 read of s: the
// predicate the transport holds to decide whether an arriving frame is this
// read's answer.
//
// THE WHOLE CORRELATION KEY SPELLS AS A PREFIX, which is why this family
// needs almost no matcher code. core/kw's MRAnswerMatcher decodes the slot
// because the 590's channel number sits at P2/P3, AFTER P1, so no prefix a
// caller can spell separates one channel's answer from another's; here the
// number is bytes 4-6, immediately after the opcode. The two rows then differ
// in one thing only — the printed length — and each arm lives in its own
// codec file. Both rest on A5: if the radio space-pads the number instead the
// matcher MISSES and the read times out, which is the fail-safe direction —
// it cannot mis-attribute another channel's answer.
//
// AN UNCONFIGURED LAYOUT OR AN UNRESOLVED SLOT MATCHES NOTHING, which is the
// fail-closed direction: a predicate that matched everything would hand the
// parser the first frame off the wire.
func (l Layout) MA0AnswerMatcher(s Slot) func(frame []byte) bool {
	never := func([]byte) bool { return false }
	if !l.Configured() || s.IsZero() {
		return never
	}
	prefix := ma0Prefix + s.String()
	switch l.book {
	case kw.Book890:
		return ma0AnswerMatcher890(prefix)
	case kw.Book990:
		return kw.PrefixLenMatcher(prefix, ma990Len)
	}
	return never
}

// checkSlot refuses a slot no layout constructed, and one whose class does
// not agree with this row's own slot space.
//
// THE CLASS IS RE-RESOLVED rather than trusted: a Slot minted by the other
// row would carry a class this row might not name, and the two rows'
// domains are identical today only because both books print the same three
// classes.
func (l Layout) checkSlot(s Slot) error {
	if s.IsZero() {
		return fmt.Errorf("this names no slot; a Slot comes from Layout.NewSlot and the zero value resolved against no radio")
	}
	if got := l.classOf(s.number); got != s.class {
		return fmt.Errorf("slot %d is %v on the %s and the record carries %v", s.number, got, l.model, s.class)
	}
	return nil
}

// checkRecordCommon applies the refusals that are the same sentence on both
// grids: an unconfigured layout, a slot no layout minted, and the empty
// record no builder here may write.
func (l Layout) checkRecordCommon(rec Record) error {
	if !l.Configured() {
		return fmt.Errorf("this layout is unconfigured and describes no radio")
	}
	if err := l.checkSlot(rec.Slot); err != nil {
		return err
	}
	if rec.Empty {
		// DECISION 15's STANDING no erase RULE. Both books print a
		// dedicated deletion command, MA5 (890:3305-3311, 990:3042-3047),
		// and this programme declines to build it; a Set assembled from a
		// record the parser marked empty would be that erase in another
		// costume.
		return fmt.Errorf("this record is a channel the parser reported unassigned, and no builder here writes one — the standing no erase rule declines even the printed MA5 (890:3305-3311, 990:3042-3047)")
	}
	return nil
}

// checkFreq refuses a frequency wider than the printed field. field is the
// caller's P-number, because the same datum is P2 on one radio and P3 on the
// other.
//
// IT NAMES THE FIELD WIDTH AND NOT ANY RADIO'S TUNING RANGE, which neither
// book prints anywhere (A15).
func checkFreq(field string, hz uint64) error {
	if hz > ma0MaxFreqHz {
		return fmt.Errorf("%s is %d, which does not fit the printed 11 digits — the widest value that field holds is %d, and this names the FIELD WIDTH rather than any radio's tuning range, which neither book prints (A15)", field, hz, uint64(ma0MaxFreqHz))
	}
	return nil
}

// encodeFreq renders hz as the printed eleven digits. "Blank digits must be
// entered as '0'" (890:3171-3172, 990:2927-2928), which is what the
// zero-padding is.
func encodeFreq(hz uint64) string {
	return fmt.Sprintf("%0*d", ma0FreqDigits, hz)
}

// encodeToneIndex renders idx as the printed two digits.
func encodeToneIndex(idx int) string {
	return fmt.Sprintf("%0*d", ma0ToneDigits, idx)
}

// checkToneIndex refuses a TN index outside this row's own printed chart.
//
// THE CEILING IS CONSULTED FROM THE SAME PLACE AS ITS DATUM — the layout's
// own MaxToneIndex, read off that row's chart — and index 99 is NOT a tone:
// both books print it "To default" / "Default", a SETTING COMMAND ONLY
// (890:5163, 890:5166; 990:4972, 990:4974).
func (l Layout) checkToneIndex(field string, idx int) error {
	if idx < 0 || idx > int(l.maxToneIndex) {
		return fmt.Errorf("%s is %d, and the %s's TN chart prints 00 ~ %d (890:5149-5163, 990:4960-4972); an index outside its own chart is refused rather than clamped", field, idx, l.model, l.maxToneIndex)
	}
	return nil
}

// checkCTCSSIndex refuses a CN index outside this row's own printed chart.
// Neither book's CN chart has an index 50, which is why a Known tone_rx of
// 1750 Hz has no receive encoding at all and is refused on write by the
// driver (decision 8).
func (l Layout) checkCTCSSIndex(field string, idx int) error {
	if idx < 0 || idx > int(l.maxCTCSSIndex) {
		return fmt.Errorf("%s is %d, and the %s's CN chart prints 00 ~ %d (890:1354-1369, 990:1251-1264); an index outside its own chart is refused rather than clamped", field, idx, l.model, l.maxCTCSSIndex)
	}
	return nil
}

// checkToneType refuses an FM tone selector outside the four values both
// books print (890:3180-3185, 990:2915-2919).
func checkToneType(field string, b byte) error {
	if b < '0' || b > '3' {
		return fmt.Errorf("%s is %q, and both books print exactly four values there — 0 OFF, 1 Tone, 2 CTCSS, 3 Cross Tone (890:3180-3185, 990:2915-2919)", field, b)
	}
	return nil
}

// checkModeByte refuses a mode this row's OM P2 legend does not name, and
// names the two values BOTH books print "Unused" separately, because those
// two are a documented absence rather than a transcription gap.
//
// THERE IS NO MD COMMAND ON EITHER RADIO: MA0 carries no mode legend of its
// own and refers to OM's (890:3174-3175, 990:2907-2910), which is why the
// legend is per-row data on the layout.
func (l Layout) checkModeByte(field string, b byte) error {
	if b == '0' || b == '8' {
		return fmt.Errorf("%s is %q, which BOTH books print \"Unused\" (890:3977, 890:3985; 990:3707, 990:3715) — it names no mode a channel can be in", field, b)
	}
	if _, ok := l.modeNames[b]; !ok {
		return fmt.Errorf("%s is %q, which the %s's OM P2 legend does not name (890:3976-3992, 990:3706-3730)", field, b, l.model)
	}
	return nil
}

// checkName refuses a name the printed field cannot carry.
//
// THE ';' EXCLUSION IS FORCED BY THE ENVELOPE (890:92-96) and is NOT an
// assumption: a name carrying one is two frames to the radio's own parser and
// one to this codec's. The rest of the charset is A2, whose lift observes
// three characters and whose upper bound at 0x7E remains open.
func checkName(field, name string) error {
	if len(name) > ma0MaxNameLen {
		return fmt.Errorf("%s is %d characters, and both books print up to 10 characters (890:3208-3209, 990:2955-2956)", field, len(name))
	}
	for i := 0; i < len(name); i++ {
		if b := name[i]; b == ';' {
			return fmt.Errorf("%s contains ';' at character %d, which the envelope reserves as the frame terminator (890:92-96) — such a name is two frames to the radio's own parser", field, i+1)
		} else if b < 0x20 || b > 0x7e {
			return fmt.Errorf("%s byte %d is %#02x, outside the printable ASCII this design writes (A2)", field, i+1, b)
		}
	}
	return nil
}

// parseName checks a name window against checkName's domain and right-trims
// the trailing spaces (A1). It reports the window's bytes verbatim in the
// refusal, so out-of-domain data is REPORTED rather than repaired.
func parseName(field string, window []byte) (string, error) {
	if err := checkName(field, string(window)); err != nil {
		return "", err
	}
	return strings.TrimRight(string(window), " "), nil
}

// decodeDigits reads a fixed-width decimal field. A SPACE IS NOT A DIGIT
// here: this family spells its numeric fields with digits in both directions
// (A5 for the channel number, "blank digits must be entered as 0" for the
// frequencies), so a space is out-of-domain data to report rather than a
// convention to absorb — which is the difference from the 590's MC field.
func decodeDigits(field string, window []byte) (uint64, error) {
	var v uint64
	for i, b := range window {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("%s byte %d is %q, and this field is printed as %d decimal digits", field, i+1, b, len(window))
		}
		v = v*10 + uint64(b-'0')
	}
	return v, nil
}

// decodeFlag reads a byte both books print as exactly 0 and 1.
func decodeFlag(field string, b byte) (bool, error) {
	switch b {
	case '0':
		return false, nil
	case '1':
		return true, nil
	}
	return false, fmt.Errorf("%s is %q, and the chart prints only 0 and 1 there", field, b)
}

// flagByte renders a boolean as the 0/1 both charts print.
func flagByte(v bool) byte {
	if v {
		return '1'
	}
	return '0'
}

// isBlankWindow is the EMPTY-CHANNEL PREDICATE'S SHAPE, shared because the
// sentence is the same on both radios and only the window differs: "When
// reading a blank channel, parameters P2 to P12 becomes blank"
// (890:3215-3216) and "parameters P2 to P18 become blank" (990:2962-2963).
//
// IT ACCEPTS ALL SPACES OR ALL ASCII '0', and the disjunction is A6 wearing
// its consequence. Neither MA0 block defines "blank"; both books define it
// for QR — "this setting is space" (890:4362-4363), "this setting is blank
// <0x20>" (990:4082) — and carrying that here is the assumption. A radio that
// blanked with zeros instead would otherwise read as a real channel tuned to
// 0 Hz, and zero hertz is not a channel on either chart.
//
// A MIXED WINDOW IS NOT BLANK, and falls through to the field parse, where it
// is REPORTED with the field and byte that offended rather than silently
// treated as an unused slot.
func isBlankWindow(window []byte) bool {
	return allBytes(window, ' ') || allBytes(window, '0')
}

// allBytes reports whether every byte of window is b. It also serves the
// zeroed-secondary-side test (A16), which is the same shape over a different
// window.
func allBytes(window []byte, b byte) bool {
	for _, got := range window {
		if got != b {
			return false
		}
	}
	return true
}

// parseSlotField decodes the three channel-number digits both grids put
// immediately after the opcode.
func (l Layout) parseSlotField(frame []byte) (Slot, error) {
	n, err := decodeDigits("P1, the channel number", frame[ma0SlotOff:ma0SlotOff+ma0SlotDigits])
	if err != nil {
		// A5 IS THE ANSWER-DIRECTION NARROWING and its failure direction is
		// safe: a space-padded answer misses the matcher and times out
		// rather than being mis-attributed. Reaching this refusal means a
		// frame got past the matcher with a number this codec cannot read.
		return Slot{}, fmt.Errorf("%v — both books print P1 as three cells over 000 ~ 119 (890:3166-3169, 990:2893-2896) and A5 records that the answer spells it back zero-padded", err)
	}
	s, err := l.NewSlot(int(n))
	if err != nil {
		return Slot{}, fmt.Errorf("P1 is %03d: %v", n, err)
	}
	return s, nil
}
