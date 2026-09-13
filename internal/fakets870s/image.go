// SPDX-License-Identifier: GPL-3.0-or-later

package fakets870s

// Image is a factory image: a function returning a freshly populated set of
// memory records. EACH CALL MUST RETURN AN INDEPENDENT MAP, so that multiple
// *Radio instances never share mutable state — the same contract
// internal/fakets480's Image documents, and for the same reason.
type Image func() map[recordKey]MemState

var _ Image = DefaultImage

// THE EVIDENCE POSTURE OF EVERY BYTE BELOW — doc.go's register entry THE
// DEFAULT IMAGE'S RECORD COMPOSITION. Every byte value in a record this file
// builds is the printed example frequency or a printed legend value; no MR
// or MW frame is printed as a literal anywhere in this book, so the
// CROSS-FIELD COMBINATION is a synthetic composition, not an observed or
// factory-default record.

// printedExampleFreq is the frequency Format 4 prints as a worked value:
// "Ex.: 00014230000 is 14.230 MHz." (ts870s:8267-8270).
const printedExampleFreq = "00014230000"

// The mode nibbles used below, from Format 2's legend (ts870s:8256-8262).
const (
	modeUSB = '2'
	modeFM  = '4'
)

// The subtone chart's first and last printed indices (ts870s:8296-8299 for
// the field, the 39-row table itself elsewhere in this book — index 39 is
// the 1750 Hz European repeater-access tone).
const (
	toneIndexFirst = "01"
	toneIndexLast  = "39"
)

// defaultRecord is one unremarkable populated record: the printed example
// frequency, FM, tone off.
func defaultRecord() MemState {
	return MemState{
		Freq:     printedExampleFreq,
		Mode:     modeFM,
		Lockout:  '0',
		ToneMode: '0',
		ToneNo:   toneIndexFirst,
	}
}

// DefaultImage is the image New uses when no WithFactoryImage option is
// given: two ordinary memory channels' RX/Start halves, and NOTHING ELSE.
//
// MINIMAL BY DESIGN: at least one channel is populated so the fleet's
// read-every-registered-model pins are non-vacuous against this row; every
// printed value of both live BINARY legends — the lockout's two and the tone
// mode's two (ts870s:8256-8262 area) — appears, so no read behaviour on
// either is a fixture accident; and nothing else is populated, because this
// book's vacant-channel sentence (ts870s:9101-9104) is a documentary claim
// this image must not turn into a fixture accident by over-populating.
//
// ONLY THE RX/START HALF SHIPS. Unlike internal/fakets480's row, the TX/End
// half (P1=1) IS a published field on this row for channels 00-98 (matrix
// §2, FieldTxFrequency: rw) — but no image byte for it is printed as a
// literal either, so a test that needs one writes it with an MW
// (TestMW_TheTwoHalvesAreSeparateRecords) or uses WithSplitChannel, exactly
// as internal/fakets590's own image does for its own split half.
func DefaultImage() map[recordKey]MemState {
	img := map[recordKey]MemState{}

	// Channel 00 — the plain one: FM, tone off, the first tone index.
	img[recordKey{channel: 0, half: HalfRXOrStart}] = defaultRecord()

	// Channel 01 — the SECOND printed value of the lockout and of the tone
	// mode, USB instead of FM, and the LAST tone index, so no record in this
	// image repeats another on the axes that matter.
	second := defaultRecord()
	second.Mode = modeUSB
	second.Lockout = '1'  // "1: Locked out" (Format 10)
	second.ToneMode = '1' // "1: ON" (Format 1, ts870s:8256)
	second.ToneNo = toneIndexLast
	img[recordKey{channel: 1, half: HalfRXOrStart}] = second

	return img
}
