// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "fmt"

// ToneMode is the memory record's P7 byte: what, if anything, the channel
// does with the two tone indices beside it.
//
// P7 GIVES THE MODE; P8 AND P9 GIVE INDEPENDENT TX AND RX INDICES. That is
// the tone_mode / tone_tx / tone_rx triple, and the 590 pair's fourth value
// makes the independence explicit — IF P14's note spells it out: "When Cross
// Tone is ON, the transceiver transmits on the Tone frequency and receives
// on the CTCSS frequency" (590:1167-1171).
type ToneMode byte

// The four P7 values, at the bytes the books print.
const (
	// ToneModeOff is "0: TONE/CTCSS OFF" (590:1550) and "0: OFF" (480:964).
	ToneModeOff ToneMode = '0'
	// ToneModeTone is "1: TONE ON" (590:1551) and "1: TONE" (480:964).
	ToneModeTone ToneMode = '1'
	// ToneModeCTCSS is "2: CTCSS ON" (590:1552) and "2: CTCSS" (480:964).
	ToneModeCTCSS ToneMode = '2'
	// ToneModeCross is "3: Cross Tone ON" (590:1553). THE TS-480 DOES NOT
	// PRINT IT: its P7 legend stops at 2 (480:964), which is the whole of
	// the ToneModesThree / ToneModesFour axis.
	ToneModeCross ToneMode = '3'
)

// Wire returns t's single wire byte.
func (t ToneMode) Wire() byte { return byte(t) }

// String renders t for refusals and logs.
func (t ToneMode) String() string {
	switch t {
	case ToneModeOff:
		return "off"
	case ToneModeTone:
		return "tone"
	case ToneModeCTCSS:
		return "CTCSS"
	case ToneModeCross:
		return "cross tone"
	default:
		return fmt.Sprintf("ToneMode(%q)", byte(t))
	}
}

// The printed tone-index domains, which are the SAME on both books.
//
// TN prints "00 ~ 42" (590:2291, 480:1557) and CN "00 ~ 41" (590:411,
// 480:337). The 590SG prints both index-to-Hz tables in full (590:2296-2306
// and 590:416-426); the TS-480 prints neither, referring the reader to two
// pages of its instruction manual (480:1559-1560, 480:339-340), so the
// RANGES agree while the 480's MAPPING is not established and is not
// borrowed across the model boundary. What this codec bounds is the index,
// which both books print.
//
// A21 IS WHAT MAKES THIS A REFUSAL RATHER THAN A CLAMP. The 590 pair print
// "An entered value of 43 or higher results in an error" for TN (590:2309)
// and nothing at all for CN; the 480 prints nothing for either, and what the
// rule does INSIDE an MW record is unprinted on all three rows. Refusing is
// this programme's choice, and it is the choice that never silently stores a
// tone the user did not ask for.
const (
	MinToneIndex  = 0
	MaxToneIndex  = 42
	MinCTCSSIndex = 0
	MaxCTCSSIndex = 41
)

// ValidToneMode reports whether t is a P7 value THIS LAYOUT'S radio prints.
//
// AGAINST THE RECEIVER, never a package-level range: the fourth value is the
// 590 pair's alone, and a range check would admit a cross-tone record on a
// TS-480 whose book has no such setting.
//
// A zero Layout prints nothing and admits nothing.
func (l Layout) ValidToneMode(t ToneMode) bool {
	switch l.toneModes {
	case ToneModesFour:
		return t == ToneModeOff || t == ToneModeTone || t == ToneModeCTCSS || t == ToneModeCross
	case ToneModesThree:
		return t == ToneModeOff || t == ToneModeTone || t == ToneModeCTCSS
	case ToneModesTwo:
		return t == ToneModeOff || t == ToneModeTone
	default:
		return false
	}
}

// toneModeText renders this layout's own P7 domain for a refusal, so the
// message says what the radio in front of the user prints rather than what
// the family prints somewhere.
func (l Layout) toneModeText() string {
	switch l.toneModes {
	case ToneModesFour:
		return `'0' TONE/CTCSS OFF, '1' TONE ON, '2' CTCSS ON or '3' Cross Tone ON (590:1549-1553)`
	case ToneModesThree:
		return `'0' OFF, '1' TONE or '2' CTCSS (480:964) — this radio's book prints no cross tone`
	case ToneModesTwo:
		return `'0' OFF or '1' ON (ts570-capability-matrix.md §1.4) — this radio's book prints no CTCSS or cross tone`
	default:
		return "a P7 value this layout declares, but it declares none"
	}
}
