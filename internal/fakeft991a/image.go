// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

import "fmt"

// Image is a factory image: a function returning a freshly populated set of
// memory slots. EACH CALL MUST RETURN AN INDEPENDENT MAP, so that multiple
// *Radio instances — or repeated New() calls with one Image value — never
// share mutable slot state. Every Image in this package is a plain function
// building its map from scratch, which is what makes that hold; a caller
// supplying its own (WithFactoryImage) owes the same property, and
// TestDefaultImage_EachCallIsIndependent pins it for DefaultImage — with
// TestTwoRadiosFromOneImageDoNotAlias pinning what a shared map would actually
// break: a write to one live radio showing up in another.
type Image func() map[string]MemState

// The compile-time proof that this package's own image satisfies the contract
// callers are typed against.
var _ Image = DefaultImage

// Mode nibble codes used by the fixtures below, from the legend printed
// identically beside five commands (ft991a_layout.txt:973-975, 1006-1008,
// 1044-1046, 789-791, 1124-1126).
const (
	modeLSB = '1'
	modeUSB = '2'
	modeFM  = '4'
)

// encodeFreqDigits converts hz to the 9-digit ASCII P2 field, refusing values
// that would need more than 9 digits.
//
// It does NOT enforce this radio's frequency RANGE, and that omission is
// deliberate: the only range this manual prints anywhere is FA/FB's on folio
// 9, and reading it as the MEMORY-storable range is an assumption the DRIVER's
// register carries. A fake that enforced it at the wire would assert that
// assumption as though it were the radio's own behaviour, which is precisely
// what this package exists not to do. The nine-digit width, by contrast, is
// the counted chart (core/cat/ft991a/testdata/mt-vectors.golden).
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 999_999_999 {
		return "", fmt.Errorf("fakeft991a: frequency %d Hz needs more than 9 digits", hz)
	}
	return fmt.Sprintf("%09d", hz), nil
}

// validModeBuildByte reports whether m is a mode nibble a BUILDER may emit:
// validModeWireByte without the '0' placeholder, which parsers accept and
// builders must not produce (the dialect's register entry "THE cat.ModeUnset
// MEMBER OF THE MODE TABLE", cited).
func validModeBuildByte(m byte) bool { return validModeWireByte(m) && m != '0' }

// defaultState builds one unremarkable populated slot: the given frequency and
// mode, no clarifier offset, BOTH clarifier flags off, CTCSS off, simplex, the
// answer kind, the schema's P11 and no tag.
//
// It panics on an invalid argument, deliberately: every call is a
// compile-time-known fixture constant in this package, so a bad one is a
// programming error in a test fixture and must stop the programme rather than
// be threaded through an error return that no caller could act on.
func defaultState(freqHz uint64, mode byte) MemState {
	freq, err := encodeFreqDigits(freqHz)
	if err != nil {
		panic(err)
	}
	if !validModeBuildByte(mode) {
		panic("fakeft991a: invalid mode byte in a fixture constant")
	}
	return MemState{
		Freq:     freq,
		ClarSign: '+',
		ClarMag:  "0000",
		RXClar:   false,
		TXClar:   false,
		Mode:     mode,
		Kind:     kindMemory,
		CTCSS:    '0',
		Shift:    '0',
		// P11 left zero: the answer builder reads that as the schema's fixed
		// '0' (see MemState.P11).
		Tag: "",
	}
}

// taggedState is defaultState with a tag.
func taggedState(freqHz uint64, mode byte, tag string) MemState {
	s := defaultState(freqHz, mode)
	if len(tag) > tagFieldLen || !validTagField([]byte(tag)) {
		panic("fakeft991a: invalid tag in a fixture constant")
	}
	s.Tag = tag
	return s
}

// pmsSlot returns a PMS slot's 3-byte wire form for a pair number (1-9) and a
// half ('L' or 'U'), from the MC legend's own line: "100: P-1L 101: P-1U ~
// 116: P-9L 117: P-9U" (ft991a_layout.txt:916). So pmsSlot(1, 'L') is "100"
// and pmsSlot(9, 'U') is "117" — DECIMAL CHANNEL NUMBERS continuing the memory
// range.
//
// IT IS THE CELL A COPY FROM internal/fakeft891 GETS WRONG. That radio's
// legend spells its PMS slots "P1L - P9U (PMS)", as do the FTdx10's and the
// FT-710's, and eighteen such strings would be eighteen non-slots here rather
// than a compile error — a silent empty bank. TestPMSSlotSpelling holds the
// four corners.
func pmsSlot(pair int, half byte) string {
	n := pmsLo + (pair-1)*2
	if half == 'U' {
		n++
	}
	return fmt.Sprintf("%03d", n)
}

// DefaultImage is the image New uses when no WithFactoryImage option is given:
// two memory channels and TWO PMS pairs — the first (100/101) and the last
// (116/117) — and NOTHING ELSE.
//
// MINIMAL BY DESIGN, AND CONSTRAINED BY THIS MILESTONE'S PLAN AT THREE POINTS
// (decision P14):
//
//   - AT LEAST ONE MEMORY CHANNEL IS POPULATED, so the fleet's
//     read-every-registered-model pins are non-vacuous against this radio.
//   - AT LEAST ONE PMS SLOT IS POPULATED. The PMS bank is where this radio is
//     unlike every registered sibling — decimal channel numbers where theirs
//     carry a token form — so a default image that left it empty would leave
//     the novelty unexercised everywhere the default fake is read. TWO PAIRS
//     ARE HERE, the FIRST and the LAST, so the numbering's two ends (100 and
//     117) both cross the wire while the seven pairs between them stay EMPTY —
//     which is what leaves an absent PMS slot for the empty-slot and
//     create-on-Set pins to name. Nine populated pairs would make every
//     "this PMS slot is empty" assertion unreachable.
//   - NO SLOT OUTSIDE 001-117, because this radio has none, and NO DCS-STATE
//     CHANNEL: P8 '3' and '4' live in WithDCSChannels so that the default image
//     round-trips through every fleet pin predating the five-state vocabulary.
//
// Every FT-991A test that needs a populated channel names the one it needs, and
// a fake that shipped 99 populated memories would make every "this slot is
// empty" assertion a fixture accident rather than a property.
//
// THE CONTENT IS INVENTED — doc.go's register entry THE DEFAULT IMAGE'S
// CONTENT IS INVENTED. No FT-991A's factory memory contents have been read by
// this project; these are placeholders, and what they exist to provide is
// SHAPE (a plain memory channel, a tagged memory channel, a populated PMS
// pair), not plausible data.
func DefaultImage() map[string]MemState {
	slots := map[string]MemState{
		// 7.000000 MHz LSB, no tag. The frequency is internal/fakeradio's,
		// which took it from the FT-710's operation manual; nothing
		// establishes it for the FT-991A, and it is a placeholder here.
		"001": defaultState(7_000_000, modeLSB),
		// 14.250000 MHz USB, tagged — the channel that makes the tag field
		// non-empty across this image.
		"002": taggedState(14_250_000, modeUSB, "TWENTY"),
	}

	type band struct {
		pair     int // 1-9
		lowerHz  uint64
		upperHz  uint64
		lowerMod byte
		upperMod byte
	}
	// Plausible IARU Region 1 amateur band edges — placeholders, not sourced
	// from any programmed radio. The FIRST and LAST pairs only, deliberately:
	// see this function's doc comment for why the seven between them are left
	// empty.
	bands := []band{
		{1, 1_810_000, 2_000_000, modeLSB, modeLSB},   // P-1L/P-1U = 100/101, 160m
		{9, 28_000_000, 29_700_000, modeUSB, modeUSB}, // P-9L/P-9U = 116/117, 10m
	}
	for _, b := range bands {
		slots[pmsSlot(b.pair, 'L')] = defaultState(b.lowerHz, b.lowerMod)
		slots[pmsSlot(b.pair, 'U')] = defaultState(b.upperHz, b.upperMod)
	}
	return slots
}
