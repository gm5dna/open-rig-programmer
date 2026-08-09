// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

// Image is a factory image: a function returning a freshly populated set of
// memory slots. EACH CALL MUST RETURN AN INDEPENDENT MAP, so that multiple
// *Radio instances — or repeated NewD()/NewMP() calls with one Image value —
// never share mutable slot state. Every Image in this package is a plain
// function building its map from scratch, which is what makes that hold; a
// caller supplying its own (WithFactoryImage) owes the same property, and
// TestDefaultImage_EachCallIsIndependent pins it for DefaultImage — with
// TestTwoRadiosFromOneImageDoNotAlias pinning what a shared map would actually
// break: a write to one live radio showing up in another.
//
// ONE IMAGE TYPE AND ONE DEFAULT IMAGE FOR BOTH MODELS, deliberately. The
// manual's slot legends are printed once for both radios (matrix §1.3, six
// legends, none model-qualified), so a per-model image would claim a difference
// the evidence says does not exist — the same reasoning that gives the two
// models one settings descriptor (plan D4).
type Image func() map[string]MemState

// The compile-time proof that this package's own image satisfies the contract
// callers are typed against.
var _ Image = DefaultImage

// Kind digit codes for the ANSWER direction, per the combined record's P7
// legend "Read: 0: VFO 1: Memory" (layout 1324).
const (
	// kindMemory is what every populated slot in this fake answers with —
	// memory channels, PMS pair members, 5 MHz channels and EMG alike. For
	// memory channels that is the legend read plainly; for the other three it
	// is doc.go's ASSUMED register entries 2 and 3, because the legend has
	// exactly two members and no manual statement says which of them a PMS, 5xx
	// or EMG slot answers with.
	//
	// Note what this fake does NOT do: fakeradio serves '5' (PMS) on the
	// FT-710's MR answer, whose own P7 table runs 0-5 and has a member for it.
	// This radio's combined record documents two values, so inventing a third
	// would produce a frame core/cat would rightly refuse to parse.
	kindMemory = '1'
)

// Mode nibble codes used by the fixtures below, from the legend printed
// identically beside five commands (layout 1240-1243, 1089-1091, 1286-1288,
// 1321-1323, 1361-1363).
const (
	modeLSB = '1'
	modeUSB = '2'
)

// defaultState builds one unremarkable populated slot: the given frequency and
// mode, no clarifier offset, clarifier flags off, CTCSS off, simplex, the
// answer kind, the schema's P11, and no tag.
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
		panic("fakedx101: invalid mode byte in a fixture constant")
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

// DefaultImage is the image NewD and NewMP use when no WithFactoryImage option
// is given: M-01 populated, the nine PMS pairs P1L-P9U populated, and NOTHING
// ELSE — no 5 MHz bank and no EMG channel (With5xx and WithEMG add those), and
// no region concept anywhere (doc.go's divergence list).
//
// MINIMAL BY DESIGN. Every FTdx101 test that needs a populated channel names
// the one it needs, and a fake that shipped 99 populated memories would make
// every "this slot is empty" assertion a fixture accident rather than a
// property. The 5xx/EMG absence additionally makes the default fake exercise
// core/driver/ftdx101's discovery walk in its ordinary case: ~100 probes, every
// one answered "?;", no discovered banks.
//
// THE CONTENT IS INVENTED — doc.go register entry 14. No FTdx101's factory
// memory contents have been read by this project, of either model; these are
// placeholders carried over from fakeradio's own invented values, and what they
// exist to provide is SHAPE (a populated memory channel, a populated PMS pair),
// not plausible data.
func DefaultImage() map[string]MemState {
	slots := map[string]MemState{
		// M-01, 7.000000 MHz LSB. The frequency is fakeradio's, which took it
		// from the FT-710's operation manual; nothing establishes it for either
		// FTdx101, and it is a placeholder here.
		"001": defaultState(7_000_000, modeLSB),
	}

	type band struct {
		pair     int // 1-9
		lowerHz  uint64
		upperHz  uint64
		lowerMod byte
		upperMod byte
	}
	// Plausible IARU Region 1 amateur band edges, one PMS pair per common
	// HF/6m band — placeholders, not sourced from any programmed radio, and
	// carrying NO region claim (this package has no ImageUK/ImageUS pair; see
	// doc.go's divergence list).
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

// pmsSlot returns a PMS slot's 3-byte wire form, e.g. pmsSlot(1, 'L') == "P1L".
func pmsSlot(pair int, half byte) string {
	return string([]byte{'P', byte('0' + pair), half})
}

// fiveMHzSlot returns a 5 MHz ("5xx") slot's 3-byte wire form for a channel
// NUMBER, e.g. fiveMHzSlot(3) == "503".
//
// The number is the wire number, not an ordinal into a bank: the 501..599
// numbering is the DIALECT's ASSUMED register entry
// ("SlotSpace.SixtyLo/SixtyHi = 501/599", cited), and taking wire numbers here
// keeps this package from re-deriving it — a fixture names the codes it wants
// and this function only spells them.
func fiveMHzSlot(n int) string {
	return string([]byte{'5', byte('0' + (n/10)%10), byte('0' + n%10)})
}

// sparseFiveMHzChannels is the 5 MHz set With5xx populates, and it is
// DELIBERATELY SPARSE AND NON-CONTIGUOUS: the first channel, a gap, one
// mid-bank channel, a long gap, and the declared ceiling.
//
// That shape is the point. core/driver/ftdx101's discovery walks the WHOLE
// declared 5xx range with no contiguity assumption, no first-rejection
// termination and no sentinel bound (its doc.go, "Discovery walks the WHOLE
// declared range"; matrix §3.4) — precisely because the FT-710's termination
// rules are that radio's hardware facts. A contiguous fixture would let a walk
// that stopped at the first "?;" pass anyway; this one fails it at 503, and a
// walk that missed the ceiling fails it at 599.
//
// It is NOT a claim about either radio's inventory (doc.go register entry 14):
// no FTdx101 this project has touched has a 5 MHz bank at all, because this
// project has touched none.
var sparseFiveMHzChannels = []int{501, 503, 599}

// --- Options that populate the discoverable banks ---
//
// Options rather than region images: see doc.go's divergence list. fakeradio's
// ImageUK/ImageUS pair encodes an FT-710 HARDWARE finding about which variants
// have a 5xx bank; this project has no such evidence about any FTdx101, so
// there is nothing to model a region on, and core/driver/ftdx101 deliberately
// implements no RegionReporter (its doc.go, "RegionReporter is NOT
// implemented").

// With5xx populates the 5 MHz bank with sparseFiveMHzChannels (see its doc
// comment for why that set is sparse), at invented placeholder frequencies
// stepping 20 kHz from 5.260 MHz in USB — fakeradio's own placeholder scheme,
// and a placeholder here too (doc.go register entry 14).
//
// Overlay semantics, like WithSlot: it adds to whatever image is already
// present, so it must be given AFTER any WithFactoryImage in the same
// constructor call.
func With5xx() Option {
	return func(r *Radio) {
		for i, n := range sparseFiveMHzChannels {
			hz := uint64(5_260_000 + i*20_000)
			r.slots[fiveMHzSlot(n)] = defaultState(hz, modeUSB)
		}
	}
}

// WithEMG populates the emergency channel at 5.1675 MHz USB — the well-known
// conventional Alaska emergency frequency, used here as a plausible placeholder
// only, exactly as fakeradio and fakedx10 use it. Nothing establishes what a
// real FTdx101's EMG channel holds, or which variants have one (doc.go register
// entry 14).
//
// Overlay semantics, like WithSlot.
func WithEMG() Option {
	return func(r *Radio) {
		r.slots[slotEMGWire] = defaultState(5_167_500, modeUSB)
	}
}
