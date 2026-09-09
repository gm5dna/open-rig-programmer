// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"fmt"
	"strings"
	"testing"
)

// ma0Fields is a 57-byte MA0 answer as its EIGHTEEN PARAMETERS, so a test can
// say which one it is varying instead of counting bytes into a string
// literal. Every field is the wire spelling, because a fixture that took
// neutral values would be the codec written twice.
//
// The defaults are one ordinary populated simplex channel: 14.175 MHz FM with
// a transmit tone, scan lockout off, named. The frequency is the book's own
// worked example (990:344-345), the mode and tone bytes are printed legend
// values, and the whole frequency-2 side is the printed zeroed form
// (990:2964-2965) — so nothing here is a byte this book does not print.
type ma0Fields struct {
	slot       string // P1, three digits (990:2893-2896)
	class      byte   // P2, channel type (990:2897-2903)
	freq       string // P3, eleven digits (990:2905-2906)
	mode       byte   // P4, OM P2 legend (990:2907-2910)
	narrow     byte   // P5, FM wide/narrow (990:2912-2914)
	toneType   byte   // P6, tone function (990:2915-2919)
	tone       string // P7, two digits via TN (990:2920-2922)
	ctcss      string // P8, two digits via CN (990:2924-2926)
	txFreq     string // P9, eleven digits (990:2927-2928)
	txMode     byte   // P10 (990:2929-2931)
	txNarrow   byte   // P11 (990:2932-2934)
	txToneType byte   // P12 (990:2935-2939)
	txTone     string // P13 (990:2940-2942)
	txCTCSS    string // P14 (990:2943-2945)
	split      byte   // P15 (990:2946-2948)
	dual       byte   // P16 (990:2949-2951)
	lockout    byte   // P17, 1 OFF / 2 ON (990:2952-2954) — erratum E8
	name       string // P18, padded to the fixed ten-byte window (990:2955-2956)
}

// populatedFields is the default record for slot.
func populatedFields(slot string) ma0Fields {
	return ma0Fields{
		slot: slot, class: '0',
		freq: "00014175000", mode: '4', narrow: '0',
		toneType: '1', tone: "08", ctcss: "12",
		// The printed zeroed frequency-2 side (A16, 990:2964-2965).
		txFreq: "00000000000", txMode: '0', txNarrow: '0',
		txToneType: '0', txTone: "00", txCTCSS: "00",
		split: '0', dual: '0', lockout: '1', name: "Bench",
	}
}

// frame renders the fixture as the 57 bytes the book prints.
func (f ma0Fields) frame() string {
	var b strings.Builder
	b.WriteString("MA0")
	b.WriteString(f.slot)
	b.WriteByte(f.class)
	b.WriteString(f.freq)
	b.WriteByte(f.mode)
	b.WriteByte(f.narrow)
	b.WriteByte(f.toneType)
	b.WriteString(f.tone)
	b.WriteString(f.ctcss)
	b.WriteString(f.txFreq)
	b.WriteByte(f.txMode)
	b.WriteByte(f.txNarrow)
	b.WriteByte(f.txToneType)
	b.WriteString(f.txTone)
	b.WriteString(f.txCTCSS)
	b.WriteByte(f.split)
	b.WriteByte(f.dual)
	b.WriteByte(f.lockout)
	b.WriteString(f.name)
	b.WriteString(strings.Repeat(" ", 10-len(f.name)))
	b.WriteByte(';')
	return b.String()
}

// populatedMA0 is the default record for slot as a raw frame.
func populatedMA0(slot string) string { return populatedFields(slot).frame() }

// blankMA0 is a blank channel's answer: "MA0" + the slot + fifty spaces + ';'
// (990:2962-2963, modulo A6's reading of "blank" as ASCII space). It is the
// printed form and no byte of it is padded to make a test pass.
func blankMA0(slot string) string {
	return "MA0" + slot + strings.Repeat(" ", 50) + ";"
}

// assertFrameWidth fails unless a fixture is the 57 bytes the grid prints, so
// a mis-built fixture is reported as itself rather than as a driver failure.
func assertFrameWidth(t *testing.T, frame string) {
	t.Helper()
	if len(frame) != ma0AnswerLen {
		t.Fatalf("fixture is %d bytes, want %d (990:2919-2938): %q", len(frame), ma0AnswerLen, frame)
	}
}

// ma0Image is a radioImage answering one slot with frame.
func ma0Image(slot, frame string) radioImage {
	return radioImage{ma0Answers: map[string]string{slot: frame}}
}

// slotID renders n as this row's three-digit canonical slot identifier.
func slotID(n int) string { return fmt.Sprintf("%03d", n) }
