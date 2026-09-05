// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

import "fmt"

// Image is a factory image: a function returning a freshly populated set of
// memory slots. EACH CALL MUST RETURN AN INDEPENDENT MAP, so that multiple
// *Radio instances — or repeated New() calls with one Image value — never
// share mutable slot state. Every Image in this package is a plain function
// building its map from scratch, which is what makes that hold; a caller
// supplying its own (WithFactoryImage) owes the same property, and
// TestDefaultImage_EachCallIsIndependent pins it for DefaultImage — with
// TestTwoRadiosFromOneImageDoNotAlias pinning what a shared map would
// actually break: a write to one live radio showing up in another.
type Image func() map[string]MemState

// The compile-time proof that this package's own image satisfies the contract
// callers are typed against.
var _ Image = DefaultImage

// Mode nibble codes used by the fixtures below, from the legend printed
// identically beside three commands (ft891_layout.txt:972-974, 1007-1010,
// 1043-1046).
const (
	modeLSB = '1'
	modeUSB = '2'
)

// encodeFreqDigits converts hz to the 9-digit ASCII P2 field, refusing values
// that would need more than 9 digits.
//
// It does NOT enforce this radio's frequency RANGE, and that omission is
// deliberate: the only range this manual prints anywhere is FA/FB's
// "000030000 - 056000000 (Hz)" on folio 9, and reading it as the
// MEMORY-storable range is an assumption the DRIVER register carries (its
// entry "MinFreqHz 30_000 / MaxFreqHz 56_000_000 — THE FA/FB RANGE READ AS
// THE MEMORY-STORABLE RANGE"). A fake that enforced it at the wire would
// assert that assumption as though it were the radio's own behaviour, which
// is precisely what this package exists not to do. The nine-digit width, by
// contrast, is the counted chart (provenance.md §MT).
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 999_999_999 {
		return "", fmt.Errorf("fakeft891: frequency %d Hz needs more than 9 digits", hz)
	}
	return fmt.Sprintf("%09d", hz), nil
}

// validModeBuildByte reports whether m is a mode nibble a BUILDER may emit:
// validModeWireByte without the '0' placeholder, which parsers accept and
// builders must not produce (the dialect's register entry "THE cat.ModeUnset
// MEMBER OF THE MODE TABLE", cited).
func validModeBuildByte(m byte) bool { return validModeWireByte(m) && m != '0' }

// defaultState builds one unremarkable populated slot: the given frequency and
// mode, no clarifier offset, the RX clarifier off, CTCSS off, simplex, the
// answer kind, the schema's P5, the TAG display OFF and no tag.
//
// It panics on an invalid argument, deliberately: every call is a
// compile-time-known fixture constant in this file, so a bad one is a
// programming error in a test fixture and must stop the programme rather than
// be threaded through an error return that no caller could act on.
func defaultState(freqHz uint64, mode byte) MemState {
	freq, err := encodeFreqDigits(freqHz)
	if err != nil {
		panic(err)
	}
	if !validModeBuildByte(mode) {
		panic("fakeft891: invalid mode byte in a fixture constant")
	}
	return MemState{
		Freq:     freq,
		ClarSign: '+',
		ClarMag:  "0000",
		RXClar:   false,
		Mode:     mode,
		Kind:     kindMemory,
		CTCSS:    '0',
		Shift:    '0',
		// P5 left zero: the answer builder reads that as the schema's fixed
		// '0' (see MemState.P5).
		TagDisplay: false,
		Tag:        "",
	}
}

// taggedState is defaultState with a tag and the TAG display flag ON.
//
// It exists so that the DEFAULT image carries both values of byte 28. That
// byte is this radio's one genuinely new axis — a live flag where every
// registered combined-form sibling prints "0: (Fixed)" — and an image whose
// every channel answered '0' would leave every default-fake read
// indistinguishable from an FTdx10's on exactly the axis this milestone turns
// on (TestDefaultImage_CarriesBothTagDisplayValues).
func taggedState(freqHz uint64, mode byte, tag string) MemState {
	s := defaultState(freqHz, mode)
	if len(tag) > tagFieldLen || !validTagField([]byte(tag)) {
		panic("fakeft891: invalid tag in a fixture constant")
	}
	s.Tag = tag
	s.TagDisplay = true
	return s
}

// DefaultImage is the image New uses when no WithFactoryImage option is given:
// two memory channels, the nine PMS pairs P1L-P9U, and NOTHING ELSE — no 5 MHz
// bank and no EMG channel (With5MHz and WithEMG add those), and no region
// concept anywhere (see the note above those two options).
//
// MINIMAL BY DESIGN, AND CONSTRAINED BY THIS MILESTONE'S PLAN AT BOTH ENDS:
//
//   - AT LEAST ONE MEMORY CHANNEL IS POPULATED, so the fleet's
//     read-every-registered-model pins are non-vacuous against this radio.
//   - NO 5 MHz AND NO EMG SLOT, so the default fake exercises
//     core/driver/ft891's discovery walk in its ordinary case — eleven MR
//     probes, all answered "?;", no discovered banks — and so the UI's
//     one-bank-set-per-model membership pins see the same seven banks every
//     time.
//
// Every FT-891 test that needs a populated channel names the one it needs, and
// a fake that shipped 99 populated memories would make every "this slot is
// empty" assertion a fixture accident rather than a property.
//
// THE CONTENT IS INVENTED — doc.go's register entry THE DEFAULT IMAGE'S
// CONTENT IS INVENTED. No FT-891's factory memory contents have been read by
// this project; these are placeholders, and what they exist to provide is
// SHAPE (a plain memory channel, a tagged memory channel with its TAG display
// ON, a populated PMS pair), not plausible data.
func DefaultImage() map[string]MemState {
	slots := map[string]MemState{
		// M-01, 7.000000 MHz LSB, no tag and the TAG display off. The
		// frequency is internal/fakeradio's, which took it from the FT-710's
		// operation manual; nothing establishes it for the FT-891, and it is
		// a placeholder here.
		"001": defaultState(7_000_000, modeLSB),
		// M-02, 14.250000 MHz USB, tagged, with the TAG display ON — the
		// channel that makes byte 28 non-constant across this image.
		"002": taggedState(14_250_000, modeUSB, "TWENTY"),
	}

	type band struct {
		pair     int // 1-9
		lowerHz  uint64
		upperHz  uint64
		lowerMod byte
		upperMod byte
	}
	// Plausible IARU Region 1 amateur band edges, one PMS pair per common HF
	// band — placeholders, not sourced from any programmed radio, and all
	// inside the only frequency range this manual prints (FA/FB, folio 9).
	bands := []band{
		{1, 1_810_000, 2_000_000, modeLSB, modeLSB},   // 160m
		{2, 3_500_000, 3_800_000, modeLSB, modeLSB},   // 80m
		{3, 7_000_000, 7_200_000, modeLSB, modeLSB},   // 40m
		{4, 10_100_000, 10_150_000, modeUSB, modeUSB}, // 30m
		{5, 14_000_000, 14_350_000, modeUSB, modeUSB}, // 20m
		{6, 18_068_000, 18_168_000, modeUSB, modeUSB}, // 17m
		{7, 21_000_000, 21_450_000, modeUSB, modeUSB}, // 15m
		{8, 24_890_000, 24_990_000, modeUSB, modeUSB}, // 12m
		{9, 28_000_000, 29_700_000, modeUSB, modeUSB}, // 10m
	}
	for _, b := range bands {
		slots[pmsSlot(b.pair, 'L')] = defaultState(b.lowerHz, b.lowerMod)
		slots[pmsSlot(b.pair, 'U')] = defaultState(b.upperHz, b.upperMod)
	}
	return slots
}

// pmsSlot returns a PMS slot's 3-byte wire form, e.g. pmsSlot(1, 'L') ==
// "P1L", using the legend's own spelling "P1L - P9U (PMS)"
// (ft891_layout.txt:961, 999, 1036).
func pmsSlot(pair int, half byte) string {
	return string([]byte{'P', byte('0' + pair), half})
}

// fiveMHzSlot returns a 5 MHz slot's 3-byte wire form for a channel NUMBER,
// e.g. fiveMHzSlot(3) would be "503".
//
// The number is the wire number this radio's own legend prints — "501 - 510
// (5 MHz, U.S. and U.K. version only)" (ft891_layout.txt:962) — not an ordinal
// into a bank. Unlike the FTdx10's, these bounds are TRANSCRIBED rather than
// inherited, which is why parseSlotForm refuses 511 and why nothing here
// carries an ASSUMED marker for the numbering.
func fiveMHzSlot(n int) string {
	return string([]byte{'5', byte('0' + (n/10)%10), byte('0' + n%10)})
}

// sparseFiveMHzChannels is the 5 MHz set With5MHz populates, and it is
// DELIBERATELY SPARSE AND NON-CONTIGUOUS: the first channel, a gap, one
// mid-bank channel, a longer gap, and the declared ceiling.
//
// That shape is the point. core/driver/ft891's discovery walks the WHOLE
// declared range, 501..510 in order, with no contiguity assumption, no
// first-rejection termination and no sentinel — deliberately the FTdx10's
// termination policy rather than the FT-710's, because the FT-710's rules are
// that radio's HARDWARE facts about a bank believed contiguous. A contiguous
// fixture would let a walk that stopped at the first "?;" pass anyway; this
// one fails such a walk at 502, and a walk that missed the ceiling fails it at
// 510.
//
// It is NOT a claim about any radio's inventory (doc.go's register entry THE
// DEFAULT IMAGE'S CONTENT IS INVENTED): no FT-891 this project has touched has
// a 5 MHz bank, and whether a U.K.-market unit has one at all is the driver
// register's own open entry.
var sparseFiveMHzChannels = []int{501, 505, 510}

// --- Options that populate the discoverable banks ---
//
// OPTIONS RATHER THAN REGION IMAGES. internal/fakeradio's ImageUK/ImageUS pair
// encodes an FT-710 HARDWARE finding about which variants have a 5xx bank;
// this project has no such evidence about any FT-891, and this manual's own
// qualification — "U.S. and U.K. version only" — is a fact about which unit is
// in front of you rather than about the wire vocabulary. So the two banks are
// OPTIONS a test asks for, never regions, and core/driver/ft891 answers the
// presence question by discovery instead.

// With5MHz populates the 5 MHz bank with sparseFiveMHzChannels (see its doc
// comment for why that set is sparse), at invented placeholder frequencies
// stepping 20 kHz from 5.260 MHz in USB — internal/fakeradio's own placeholder
// scheme, and a placeholder here too (doc.go's register entry THE DEFAULT
// IMAGE'S CONTENT IS INVENTED).
//
// Overlay semantics, like WithSlot: it adds to whatever image is already
// present, so it must be given AFTER any WithFactoryImage in the same New
// call.
func With5MHz() Option {
	return func(r *Radio) {
		for i, n := range sparseFiveMHzChannels {
			hz := uint64(5_260_000 + i*20_000)
			r.slots[fiveMHzSlot(n)] = defaultState(hz, modeUSB)
		}
	}
}

// WithEMG populates the emergency channel at 5.1675 MHz USB — the well-known
// conventional Alaska emergency frequency, used here as a plausible
// placeholder only, exactly as internal/fakeradio uses it. Nothing establishes
// what a real FT-891's EMG channel holds, or which variants have one.
//
// Overlay semantics, like WithSlot.
func WithEMG() Option {
	return func(r *Radio) {
		r.slots[slotEMGWire] = defaultState(5_167_500, modeUSB)
	}
}
