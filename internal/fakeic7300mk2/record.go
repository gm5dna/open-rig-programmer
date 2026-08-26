// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300mk2

import (
	"strconv"
	"strings"
)

// The memory record's geometry and vocabularies, RE-DERIVED here from
// core/civ/ic7300mk2/testdata/ic7300mk2-transcription-b.{csv,md} and
// ic7300mk2-geometry-witness.{csv,md} and from nothing else. THE HARD RULE
// (doc.go) forbids importing the production codec that carries the same
// numbers; that independence is the whole point of this package.
//
// # How the length was derived
//
// Diagram D1 on PDF page 17, captioned "• Memory channel content / Command:
// 1A 00", carries nine bracketed index blocks. Their PRINTED index counts,
// which is the column transcription-b.csv records as width_bytes, are:
//
//	①, ②   Memory channel number             2
//	③      Split and Select memory setting   1
//	④ ~ ⑧  Operating frequency setting       5
//	⑨, ⑩   Operating mode setting            2
//	⑪      Data mode and tone type settings  1
//	⑫ ~ ⑭  Repeater tone frequency setting   3
//	⑮ ~ ⑰  Tone squelch frequency setting    3
//	❹ ~ ⓱  (no printed label)                14
//	⑱ ~ ㉝  Memory name settings              16
//	                                        ---
//	                                         47
//
// The geometry witness counts only SIXTEEN byte cells actually drawn, because
// three of the nine blocks are drawn abbreviated (its STOP 9: the elision
// marks are 1, 3 and 1 cell pitches wide but stand for 3, 14 and 14 omitted
// indices, "neither equal to nor proportional to the number of cells they
// elide"). The witness records 16 against 47 and does not reconcile them.
// This package reconciles them the only way the documents allow — see doc.go's
// ASSUMED register entry 1 — by taking the printed index counts as the byte
// widths, since the elision marks are explicitly abbreviation marks and the
// witness itself says no count can be recovered from their width.
//
// The first block, ①, ② Memory channel number, is not part of the record: it
// is the two channel-address bytes a 1A 00 frame carries before the record.
// 47 - 2 = 45.
const recordLen = 45

// Field offsets within the 45-byte record, i.e. counting from the first byte
// AFTER the two channel-address bytes. Each is the running sum of the printed
// widths above.
const (
	offSplitSelect  = 0  // ③,      1 byte
	offFrequency    = 1  // ④ ~ ⑧,  5 bytes
	offMode         = 6  // ⑨, ⑩,   2 bytes
	offDataTone     = 8  // ⑪,      1 byte
	offRepeaterTone = 9  // ⑫ ~ ⑭,  3 bytes
	offToneSquelch  = 12 // ⑮ ~ ⑰,  3 bytes
	offShadow       = 15 // ❹ ~ ⓱, 14 bytes
	offName         = 29 // ⑱ ~ ㉝, 16 bytes
)

// The ❹ ~ ⓱ block's internal offsets. Page 17's grey NOTE box prints "The
// same data as ④ ~ ⑰ are stored in ❹ ~ ⓱", and ④ ~ ⑰ is exactly the fourteen
// bytes at offFrequency..offToneSquelch+2, so the block repeats their layout
// byte for byte. (ASSUMED register entry 3.)
const (
	offShadowFrequency    = offShadow + 0  // 15, 5 bytes
	offShadowMode         = offShadow + 5  // 20, 2 bytes
	offShadowDataTone     = offShadow + 7  // 22, 1 byte
	offShadowRepeaterTone = offShadow + 8  // 23, 3 bytes
	offShadowToneSquelch  = offShadow + 11 // 26, 3 bytes
)

// The two channel-address bytes. The ①, ② legend on page 17 prints exactly
// three forms and no fourth:
//
//	00 01 ~ 00 99: Memory channel 01 ~ 99
//	01 00:         Programmed scan edge P1
//	01 01:         Programmed scan edge P2
const (
	bankMemory   = 0x00 // first byte of the "00 nn" form
	bankScanEdge = 0x01 // first byte of the "01 0n" form
	edgeP1       = 0x00
	edgeP2       = 0x01
)

// hi and lo split a byte into its two nibbles, left-printed first — the order
// the geometry witness records as its nibble 1 / nibble 2 convention, and the
// order both of page 17's one-cell expansion boxes draw (SPLIT left of SELECT,
// DATA left of TONE, arrows rising straight and not crossing).
func hi(b byte) byte { return b >> 4 }
func lo(b byte) byte { return b & 0x0F }

// canonicalSlot normalises a slot string to the canonical spelling this
// package stores under: "001" … "099", "P1", "P2". Case and surrounding
// whitespace are forgiven; nothing else is. A channel number must be written
// with its three digits, because that is the form the legend's "Memory channel
// 01 ~ 99" maps onto, and "00" is not one of the three printed forms.
func canonicalSlot(addr string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(addr))
	switch s {
	case "P1", "P2":
		return s, true
	}
	if len(s) != 3 {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return "", false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 99 {
		return "", false
	}
	return s, true
}

// slotWire returns the two channel-address bytes for a canonical slot string.
func slotWire(slot string) ([2]byte, bool) {
	s, ok := canonicalSlot(slot)
	if !ok {
		return [2]byte{}, false
	}
	switch s {
	case "P1":
		return [2]byte{bankScanEdge, edgeP1}, true
	case "P2":
		return [2]byte{bankScanEdge, edgeP2}, true
	}
	n, _ := strconv.Atoi(s)
	// bcd_packed, per the ①, ② row: "the printed codes 00 01 ~ 00 99 map
	// digit for digit onto decimal channels 01 ~ 99, so each byte carries two
	// decimal digits and no hexadecimal digit above 9 is used".
	return [2]byte{bankMemory, byte(n/10)<<4 | byte(n%10)}, true
}

// slotFromWire reads the two channel-address bytes. It admits the three
// printed forms and nothing else: any other pair is not an address this radio
// knows, and a frame carrying one is answered FAIL.
func slotFromWire(b0, b1 byte) (string, bool) {
	switch b0 {
	case bankMemory:
		if hi(b1) > 9 || lo(b1) > 9 {
			return "", false // not packed BCD
		}
		n := int(hi(b1))*10 + int(lo(b1))
		if n < 1 || n > 99 {
			return "", false // "00 00" is not one of the printed forms
		}
		return string([]byte{'0' + byte(n/100), '0' + byte(n/10%10), '0' + byte(n%10)}), true
	case bankScanEdge:
		switch b1 {
		case edgeP1:
			return "P1", true
		case edgeP2:
			return "P2", true
		}
	}
	return "", false
}

// isScanEdge reports whether a canonical slot string names one of the two
// programmed scan edges.
func isScanEdge(slot string) bool { return slot == "P1" || slot == "P2" }

// ---------------------------------------------------------------------------
// Field vocabularies
// ---------------------------------------------------------------------------

// splitSelectOK checks ③. The one-cell expansion box splits the byte with a
// dotted centre line: the left nibble is SPLIT, printed "0=OFF | 1=ON"; the
// right nibble is SELECT, printed "0=OFF | 1=★1 | 2=★2 | 3=★3". The SPLIT
// column's remaining two cells are drawn as diagonally ruled blanks, so 2 and
// 3 are not values it prints.
func splitSelectOK(b []byte) bool {
	return hi(b[0]) <= 1 && lo(b[0]) <= 3
}

// frequencyOK checks ④ ~ ⑧. Page 16's "• Operating frequency" diagram carries
// ten rotated nibble labels, read left to right beneath the five-byte row:
//
//	byte 0: 10 Hz  0~9 | 1 Hz    0~9
//	byte 1: 1 kHz  0~9 | 100 Hz  0~9
//	byte 2: 100 kHz 0~9 | 10 kHz 0~9
//	byte 3: 10 MHz 0~7 | 1 MHz   0~9
//	byte 4: 1 GHz  0 (Fixed) | 100 MHz 0 (Fixed)
//
// Every nibble is therefore a decimal digit; the 10 MHz digit stops at 7, and
// the whole of byte 4 is fixed at zero.
func frequencyOK(b []byte) bool {
	for i := 0; i < 3; i++ {
		if hi(b[i]) > 9 || lo(b[i]) > 9 {
			return false
		}
	}
	if hi(b[3]) > 7 || lo(b[3]) > 9 {
		return false
	}
	return b[4] == 0x00
}

// modeOK checks ⑨, ⑩. Page 16's "• Operating mode" table prints, in its
// ①Operating mode columns, 00=LSB 01=USB 02=AM 03=CW 04=RTTY 05=FM 07=CW-R
// 08=RTTY-R — 06 is not printed, and the two remaining cells are drawn as "—".
// Its ②Filter column prints 01=FIL1 02=FIL2 03=FIL3.
func modeOK(b []byte) bool {
	switch b[0] {
	case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x07, 0x08:
	default:
		return false
	}
	return b[1] >= 0x01 && b[1] <= 0x03
}

// dataToneOK checks ⑪. Its expansion box splits the byte the same way ③'s
// does: the left nibble is DATA, printed "0=OFF | 1=ON" with its third cell a
// diagonally ruled blank; the right nibble is TONE, printed "0=OFF | 1=TONE |
// 2=TSQL".
func dataToneOK(b []byte) bool {
	return hi(b[0]) <= 1 && lo(b[0]) <= 2
}

// repeaterToneOK checks ⑫ ~ ⑭ — and accepts anything.
//
// NOTHING WHATEVER IS PRINTED under this heading. transcription-b.csv records
// the field with an empty values_verbatim and encoding "unstated", and its
// notes say so in terms: "no value, no code, no encoding statement and no
// cross-reference". The one ⓘ cross-reference in that area, to page 23's
// "Repeater tone/tone squelch frequency settings", is set under the NEXT
// heading (⑮ ~ ⑰) and was deliberately not carried across.
//
// The tempting inference is that these three bytes carry page 23's tone BCD,
// exactly like ⑮ ~ ⑰ beside them. That inference is precisely what the
// transcription refused to make, and this package does not make it either:
// rejecting a byte against a vocabulary that was never printed would be
// inventing one. See doc.go, ASSUMED register entry 4 — the entry to change
// first if a real MK2 ever contradicts this fake.
func repeaterToneOK(b []byte) bool { return true }

// toneSquelchOK checks ⑮ ~ ⑰. Page 23's "• Repeater tone/tone squelch
// frequency settings" diagram carries six rotated nibble labels, left to
// right: "Fixed digit: 0*", "Fixed digit: 0*", "100 Hz digit: 0 ~ 2", "10 Hz
// digit: 0 ~ 9", "1 Hz digit: 0 ~ 9", "0.1 Hz digit: 0 ~ 9". The first byte's
// two nibbles are printed as literal 0s, not as X.
func toneSquelchOK(b []byte) bool {
	if b[0] != 0x00 {
		return false
	}
	if hi(b[1]) > 2 || lo(b[1]) > 9 {
		return false
	}
	return hi(b[2]) <= 9 && lo(b[2]) <= 9
}

// nameCodes is the ⑱ ~ ㉝ vocabulary: every ASCII code page 18's two
// "Character codes" tables print, plus the space named in the same page's
// "ⓘUsable characters: A to Z, a to z, 0 to 9, (space), …" list.
//
// The space is the one member whose code is NOT printed (ASSUMED register
// entry 5): the ⓘ line names it as usable, both tables head their code column
// "ASCII code", and 0x20 is the space in that scheme.
//
// The resulting set turns out to be the contiguous run 0x20 … 0x7E. That is an
// OBSERVATION about the codes the tables print, not the rule used to build the
// set — the set is built from the printed codes below, one by one, so that a
// code the tables do not print could not creep in by an off-by-one on a range.
var nameCodes = func() [256]bool {
	var t [256]bool
	for c := byte(0x41); c <= 0x5A; c++ { // A ~ Z: 41 ~ 5A
		t[c] = true
	}
	for c := byte(0x61); c <= 0x7A; c++ { // a ~ z: 61 ~ 7A
		t[c] = true
	}
	for c := byte(0x30); c <= 0x39; c++ { // 0 ~ 9: 30 ~ 39
		t[c] = true
	}
	// The Symbols table, in printed order. Note 27 and 60 are both here: page
	// 18 draws an identical glyph against each (the transcription's STOP 2),
	// and both codes are accepted without either being reconciled away.
	for _, c := range []byte{
		0x21, 0x23, 0x24, 0x25, 0x26, 0x5C, 0x3F, 0x22,
		0x27, 0x60, 0x5E, 0x2B, 0x2D, 0x2A, 0x2F, 0x2E,
		0x2C, 0x3A, 0x3B, 0x3D, 0x3C, 0x3E, 0x28, 0x29,
		0x5B, 0x5D, 0x7B, 0x7D, 0x7C, 0x5F, 0x7E, 0x40,
	} {
		t[c] = true
	}
	t[0x20] = true // (space), from the ⓘUsable characters list
	return t
}()

// nameOK checks ⑱ ~ ㉝: sixteen bytes, each a printed character code. No pad
// byte is printed anywhere, so a name shorter than sixteen characters must be
// padded by the caller with the printed space; 0x00 is not a member and is
// rejected (ASSUMED register entry 7).
func nameOK(b []byte) bool {
	for _, c := range b {
		if !nameCodes[c] {
			return false
		}
	}
	return true
}

// recordField is one of the record's fields: where it starts, how wide it is,
// and what values the transcription prints for it.
type recordField struct {
	name  string
	off   int
	width int
	check func([]byte) bool
}

// recordFields is the whole 45-byte record, in order. The widths sum to
// recordLen, which TestFieldWidthsSumToTheRecordLength pins.
var recordFields = []recordField{
	{"③ Split and Select memory setting", offSplitSelect, 1, splitSelectOK},
	{"④ ~ ⑧ Operating frequency setting", offFrequency, 5, frequencyOK},
	{"⑨, ⑩ Operating mode setting", offMode, 2, modeOK},
	{"⑪ Data mode and tone type settings", offDataTone, 1, dataToneOK},
	{"⑫ ~ ⑭ Repeater tone frequency setting", offRepeaterTone, 3, repeaterToneOK},
	{"⑮ ~ ⑰ Tone squelch frequency setting", offToneSquelch, 3, toneSquelchOK},
	// ❹ ~ ⓱, "the same data as ④ ~ ⑰", validated field by field against the
	// same vocabularies. EQUALITY with ④ ~ ⑰ is NOT required: the NOTE box
	// only recommends it ("Even if the Split function is OFF, we recommend
	// that you set the same data …"). ASSUMED register entry 3.
	{"❹ ~ ⓱ (transmit side of ④ ~ ⑧)", offShadowFrequency, 5, frequencyOK},
	{"❹ ~ ⓱ (transmit side of ⑨, ⑩)", offShadowMode, 2, modeOK},
	{"❹ ~ ⓱ (transmit side of ⑪)", offShadowDataTone, 1, dataToneOK},
	{"❹ ~ ⓱ (transmit side of ⑫ ~ ⑭)", offShadowRepeaterTone, 3, repeaterToneOK},
	{"❹ ~ ⓱ (transmit side of ⑮ ~ ⑰)", offShadowToneSquelch, 3, toneSquelchOK},
	{"⑱ ~ ㉝ Memory name settings", offName, 16, nameOK},
}

// badField reports the first field of rec whose value is not one the
// transcription prints, or "" if every field is good. rec must be recordLen
// bytes long.
func badField(rec []byte) string {
	if len(rec) != recordLen {
		return "record length"
	}
	for _, f := range recordFields {
		if !f.check(rec[f.off : f.off+f.width]) {
			return f.name
		}
	}
	return ""
}

// validRecord reports whether rec is exactly recordLen bytes and every field
// carries a printed value.
func validRecord(rec []byte) bool { return badField(rec) == "" }

// broadcastPayload is the five frequency bytes the unsolicited frames carry:
// 14.250000 MHz, packed in the order frequencyOK derives — 10 Hz/1 Hz first,
// 1 GHz/100 MHz last.
//
//	10 Hz 0, 1 Hz 0    -> 00
//	1 kHz 0, 100 Hz 0  -> 00
//	100 kHz 2, 10 kHz 5 -> 25
//	10 MHz 1, 1 MHz 4  -> 14
//	1 GHz 0, 100 MHz 0 -> 00
//
// Nothing in the two artefacts describes an unsolicited frame at all, so the
// choice of payload is this package's (ASSUMED register entry 19). It is a
// well-formed frequency so that a reader which parses it gets a sane answer,
// and it is CONSTANT so that a test can recognise the flood by sight.
var broadcastPayload = []byte{0x00, 0x00, 0x25, 0x14, 0x00}
