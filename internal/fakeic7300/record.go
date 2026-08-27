// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300

import "fmt"

// The memory record's layout, DERIVED HERE from the two artefacts named in
// doc.go and from nothing else.
//
// The diagram (`• Memory content` / `Command : 1A 00`) is one band of nine
// labelled fields. Its first field, `①, ②` "Memory channel numbers", is the
// CHANNEL ADDRESS the command carries before its payload — the two bytes a read
// sends on their own — so the RECORD is the eight fields after it. Taking each
// field's width from its own printed index range, in band order:
//
//	③      Split and Select memory setting     1
//	④–⑧    Operating frequency setting         5
//	⑨, ⑩   Operating mode setting              2
//	⑪      Data mode and tone type settings    1
//	⑫–⑭    Repeater tone frequency setting     3
//	⑮–⑰    Tone squelch frequency setting      3
//	❹–⓱    (no label printed)                 14
//	⑱–㉗   Memory name settings               10
//	                                          ──
//	                                          39
//
// 39, and 41 with the two channel-address bytes in front — which is the number
// the transcription itself reaches by the same accumulation, recording the last
// field's "measured byte position" as 32-41.
//
// WHY THE PRINTED INDEX RANGES AND NOT THE DRAWN CELLS. The geometry witness
// measures the same band as 19 DRAWN regions, and says so under five separate
// STOP findings: the `④–⑧` bracket spans three drawn cells for five printed
// indices, `⑱–㉗` three for ten, and `❹–⓱` one undivided region for fourteen.
// Two of those three regions are drawn as a dashed-outline cell printing `...`
// and the third as a dotted region containing a row of dots — elision markers,
// which the transcription's own notes distinguish from byte cells ("An elision
// marker is not a byte cell and was not counted as one"). Counting drawn
// regions would therefore count each elision as one byte and yield a 17-byte
// record, which no reading of the page supports. The index ranges are the only
// count the page states in numerals, and both artefacts accumulate them the
// same way. See doc.go's ASSUMED register, entry 1.
const (
	// recordLen is the record's total width, after the two channel-address
	// bytes: 1+5+2+1+3+3+14+10.
	recordLen = 39

	offSplitSelect  = 0  // ③
	lenSplitSelect  = 1  //
	offFrequency    = 1  // ④–⑧
	lenFrequency    = 5  //
	offMode         = 6  // ⑨, ⑩
	lenMode         = 2  //
	offDataTone     = 8  // ⑪
	lenDataTone     = 1  //
	offRepeaterTone = 9  // ⑫–⑭
	lenRepeaterTone = 3  //
	offToneSquelch  = 12 // ⑮–⑰
	lenToneSquelch  = 3  //
	offRepeatBlock  = 15 // ❹–⓱
	lenRepeatBlock  = 14 //
	offName         = 29 // ⑱–㉗
	lenName         = 10 //
)

// The two nibble-coded fields' vocabularies, read off the two one-cell legends
// the page prints beneath the band.
//
// Each legend is one byte cut by a dotted mid-point rule into a left half and a
// right half. Both artefacts trace the leaders independently and both report
// the same crossing: THE PRINTED TOP-TO-BOTTOM ORDER OF THE LABELS IS THE
// REVERSE OF THE LEFT-TO-RIGHT ORDER OF THE HALVES. The left half is the byte's
// high nibble (doc.go, ASSUMED entry 2).
//
//	③  high (left)  ← "0=Split OFF, 1=Split ON"          → 0..1
//	③  low  (right) ← "0=OFF | 1= ★1 | 2= ★2 | 3= ★3"    → 0..3
//	⑪  high (left)  ← "0=Data mode OFF | 1=Data mode ON" → 0..1
//	⑪  low  (right) ← "0: OFF, 1: TONE, 2: TSQL"         → 0..2
const (
	maxSplitNibble          = 1 // ③ high: Split OFF / Split ON
	maxSelectNibble         = 3 // ③ low: OFF / ★1 / ★2 / ★3
	maxDataModeNibble       = 1 // ⑪ high: Data mode OFF / ON
	maxToneTypeNibble       = 2 // ⑪ low: OFF / TONE / TSQL
	nameLowestPrinted       = 0x20
	nameHighestPrinted      = 0x7E
	nibbleWidth             = 4
	nibbleMask         byte = 0x0F
)

// validateRecord reports whether rec is a record this fake will store: exactly
// recordLen bytes, with every field whose values the transcription prints
// carrying one of them.
//
// The fields the transcription records as `unstated` — operating frequency,
// operating mode, repeater tone frequency, tone squelch frequency — print no
// encoding and no code list at all, only a cross-reference to a section that
// was not transcribed (and, for the tone pair, one that the transcribing leg
// reports it could not find printed on any page it read). Their bytes are
// therefore accepted as they stand: this fake refuses only what the page gives
// it grounds to refuse. Same for the ❹–⓱ block, whose label and value cells the
// page leaves empty. See doc.go, ASSUMED entries 3 and 4.
func validateRecord(rec []byte) error {
	if len(rec) != recordLen {
		return fmt.Errorf("record is %d bytes, want exactly %d (③1 ④–⑧5 ⑨⑩2 ⑪1 ⑫–⑭3 ⑮–⑰3 ❹–⓱14 ⑱–㉗10)", len(rec), recordLen)
	}

	ss := rec[offSplitSelect]
	if hi := ss >> nibbleWidth; hi > maxSplitNibble {
		return fmt.Errorf("③ high nibble %X: the legend prints 0=Split OFF and 1=Split ON, and nothing else", hi)
	}
	if lo := ss & nibbleMask; lo > maxSelectNibble {
		return fmt.Errorf("③ low nibble %X: the legend prints 0=OFF, 1=★1, 2=★2, 3=★3, and nothing else", lo)
	}

	dt := rec[offDataTone]
	if hi := dt >> nibbleWidth; hi > maxDataModeNibble {
		return fmt.Errorf("⑪ high nibble %X: the legend prints 0=Data mode OFF and 1=Data mode ON, and nothing else", hi)
	}
	if lo := dt & nibbleMask; lo > maxToneTypeNibble {
		return fmt.Errorf("⑪ low nibble %X: the legend prints 0: OFF, 1: TONE, 2: TSQL, and nothing else", lo)
	}

	for i, b := range rec[offName : offName+lenName] {
		if b < nameLowestPrinted || b > nameHighestPrinted {
			return fmt.Errorf("⑱–㉗ byte %d is %02X: outside %02X-%02X, the span of the character table's printed codes plus the pad", i+1, b, nameLowestPrinted, nameHighestPrinted)
		}
	}
	return nil
}
