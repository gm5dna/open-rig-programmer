// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

// Image is a factory image: a function returning a freshly populated set
// of memory slots, as the radio ships from the factory. Each call MUST
// return an independent map, so that multiple *Radio instances (or
// repeated New() calls) never share mutable slot state. See ImageUK,
// ImageUS.
type Image func() map[string]MemState

// defaultState fills in the fixed P9 (always "00", carried implicitly by
// buildMRAnswer, not stored) and marks the slot populated; every field
// not passed explicitly here defaults to the simplex/off/CTCSS-off
// values used throughout the factory images below.
func defaultState(freqHz uint64, mode byte, kind byte) MemState {
	freq, err := encodeFreqDigits(freqHz)
	if err != nil {
		panic(err) // factory image constants are compile-time-known-valid
	}
	sign, mag, err := encodeClarifier('+', 0)
	if err != nil {
		panic(err)
	}
	if !validModeBuildByte(mode) {
		panic("fakeradio: invalid mode byte in factory image constant")
	}
	if !validKindByte(kind) {
		panic("fakeradio: invalid kind byte in factory image constant")
	}
	return MemState{
		Freq:      freq,
		ClarSign:  sign,
		ClarMag:   mag,
		RXClar:    false,
		TXClar:    false,
		Mode:      mode,
		Kind:      kind,
		CTCSS:     '0',
		Shift:     '0',
		Populated: true,
	}
}

// Mode nibble codes (reference, "Mode nibble (P6)" table).
const (
	modeLSB = '1'
	modeUSB = '2'
)

// Kind digit codes (reference, "MR" answer table, P7).
const (
	kindMemory = '1' // Memory
	kindPMS    = '5' // PMS

	// kind60mEMG is ASSUMED (doc.go register): the manual's P7 table (0-5)
	// has no distinct code for 60m/EMG channels. We report them as
	// Memory-like (kindMemory) pending M5a confirmation.
	kind60mEMG = kindMemory
)

// baseImage returns the slots common to both regions: M-01 (reference:
// "M-01 populated (7.000000 MHz LSB per operation manual)" — matches
// golden vector G4 exactly) and the 9 PMS pairs P1L-P9U, populated with
// plausible IARU IARU Region 1 amateur band edges (placeholders — not
// sourced from any specific programmed radio; see doc.go register).
func baseImage() map[string]MemState {
	slots := map[string]MemState{
		"001": defaultState(7_000_000, modeLSB, kindMemory), // M-01, 7.000000 MHz LSB (golden vector G4)
	}

	type band struct {
		pair     int    // 1-9
		lowerHz  uint64 // PxL
		upperHz  uint64 // PxU
		lowerMod byte
		upperMod byte
	}
	// Plausible band edges (placeholders), one PMS pair per common HF/6m
	// amateur band. P1L = 1.810000 MHz LSB matches golden vector G6
	// exactly.
	bands := []band{
		{1, 1_810_000, 2_000_000, modeLSB, modeLSB},   // 160m
		{2, 3_500_000, 3_800_000, modeLSB, modeLSB},   // 80m
		{3, 7_000_000, 7_200_000, modeLSB, modeLSB},   // 40m
		{4, 10_100_000, 10_150_000, modeUSB, modeUSB}, // 30m (data/CW band; USB placeholder)
		{5, 14_000_000, 14_350_000, modeUSB, modeUSB}, // 20m
		{6, 18_068_000, 18_168_000, modeUSB, modeUSB}, // 17m
		{7, 21_000_000, 21_450_000, modeUSB, modeUSB}, // 15m
		{8, 24_890_000, 24_990_000, modeUSB, modeUSB}, // 12m
		{9, 28_000_000, 29_700_000, modeUSB, modeUSB}, // 10m
	}
	for _, b := range bands {
		lower := pmsSlot(b.pair, 'L')
		upper := pmsSlot(b.pair, 'U')
		slots[lower] = defaultState(b.lowerHz, b.lowerMod, kindPMS)
		slots[upper] = defaultState(b.upperHz, b.upperMod, kindPMS)
	}
	return slots
}

func pmsSlot(pair int, half byte) string {
	return string([]byte{'P', byte('0' + pair), half})
}

// sixtyMetreChannel returns a 60m ("5xx") slot number as its 3-byte wire
// form, e.g. sixtyMetreChannel(1) == "501". ASSUMED numbering (doc.go
// register): the reference documents 5xx as region-dependent with
// "ASSUMED 501.. numbering"; the manual does not fix it.
func sixtyMetreChannel(n int) string {
	return string([]byte{'5', byte('0' + n/10), byte('0' + n%10)})
}

// ImageUK is the default factory image (WithFactoryImage(ImageUK) is
// New's default): baseImage ONLY — HW-CONFIRMED 2026-07-13 (see
// docs/hardware-notes.md §60m regional finding): Stuart's UK FT-710 has
// NO factory 5xx bank at all (front-panel confirmed: no 5-xx channels
// anywhere in the 117-slot inventory) and no EMG channel; UK 5 MHz
// operation on this radio lives in ordinary memory channels, not a
// dedicated 60m bank. This OVERTURNS the former ASSUMED design (7
// invented 501-507 placeholder channels at round 20 kHz steps from
// 5.260 MHz — never the real Ofcom-assigned UK 60m channel plan, which
// this project now knows does not exist as a separate CAT-visible bank
// on this variant). MR/MC/EMG/5xx against a UK-image radio now behave
// exactly as any other never-touched slot (MR/MC/MT all answer "?;" —
// see parser.go's handleMT map-presence rule). Tests that still want
// 60m/EMG bank coverage use ImageUS (STILL-ASSUMED — see its own doc
// comment).
func ImageUK() map[string]MemState {
	return baseImage()
}

// ImageUS is the US factory image variant: baseImage plus 60m channels
// 501-515 (the current 15-channel US allocation) at invented placeholder
// frequencies, PLUS EMG populated at 5.1675 MHz USB — the well-known
// conventional Alaska emergency frequency, used here as a plausible
// placeholder.
//
// STILL-ASSUMED pending a US-region hardware session: M5a (13/07/2026)
// characterised only Stuart's UK FT-710 (see ImageUK's doc comment and
// docs/hardware-notes.md) — no US-variant radio has been read against
// this project yet, so this image's 60m/EMG shape (channel count,
// numbering, and the exact EMG frequency) remains an invented
// placeholder, not hardware-confirmed, and is UNCHANGED by this task.
// It is deliberately kept as-is (rather than stripped like ImageUK) so
// tests that need a 60m/EMG bank to exercise still have a fixture for
// it — every caller that used ImageUK for that purpose before this task
// now uses ImageUS instead.
func ImageUS() map[string]MemState {
	slots := baseImage()
	for i := 1; i <= 15; i++ {
		hz := uint64(5_260_000 + (i-1)*20_000)
		slots[sixtyMetreChannel(i)] = defaultState(hz, modeUSB, kind60mEMG)
	}
	slots["EMG"] = defaultState(5_167_500, modeUSB, kind60mEMG)
	return slots
}
