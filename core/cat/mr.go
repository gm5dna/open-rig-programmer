// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strconv"
)

// mrReadLen is the fixed length of an MR read request: "MR" + 3-byte slot
// + ";". Golden vector G3: "MR007;".
const mrReadLen = 6

// BuildMRRead builds an MR (memory channel read) request for slot s.
// Reference: "MR — MEMORY CHANNEL READ (Read/Answer only; no Set) ... Read
// frame (6 bytes): MR P0 P0 P0 ;", golden vector G3.
//
// Any slot ParseSlot would accept is a legal read target EXCEPT the
// special "000" placeholder (Slot.IsNone): the reference marks its
// semantics UNKNOWN/ASSUMED and says "do not emit". 5xx and EMG slots are
// explicitly readable per the reference's slot table ("MR read" column: ✓
// for both) — unlike MW/MT set, a read has no write-direction
// hardware-verification concern. See readableSlot (slot.go), shared with
// BuildMTRead and AllowedCommand's MR/MT grammar checks.
func BuildMRRead(s Slot) (Command, error) {
	if !readableSlot(s) {
		return Command{}, newParseError([]byte(s.Wire()), "MR: slot must be a readable memory/PMS/60m/EMG slot, not \"000\" or invalid")
	}
	frame := make([]byte, 0, mrReadLen)
	frame = append(frame, 'M', 'R')
	frame = append(frame, s.Wire()...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// ParseMRAnswer strictly parses a 28-byte MR answer frame per the
// reference's fixed-offset position table (P1-P10). Every field is
// validated; any deviation from the documented wire form is rejected with
// a *ParseError. Golden vectors G4, G6; G7 shares this exact body layout
// (see mr_test.go's TestParseMRAnswer_G7SharedLayout and mw.go).
//
// This is a thin wrapper around parseMemoryFrame, the shared decoder also
// used by AllowedCommand's MW grammar check (allowlist.go): the manual
// documents MR's answer and MW's set frame as byte-for-byte identical
// (memdata.go's field offsets are shared by both), so the wire-level field
// validation lives in exactly one place rather than being duplicated
// between a parser and a validator.
func ParseMRAnswer(frame []byte) (MemoryData, error) {
	return parseMemoryFrame(frame, "MR")
}

// parseMemoryFrame strictly parses a 28-byte MR-answer/MW-set-shaped frame
// per the reference's fixed-offset position table (P1-P10), additionally
// checking that its first two bytes equal wantPrefix. See ParseMRAnswer's
// doc comment for why this is factored out: it is shared, unchanged,
// between ParseMRAnswer (wantPrefix "MR") and AllowedCommand's MW grammar
// check (wantPrefix "MW").
func parseMemoryFrame(frame []byte, wantPrefix string) (MemoryData, error) {
	if len(frame) != memoryFrameLen {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame must be %d bytes", wantPrefix, memoryFrameLen))
	}
	if frame[0] != wantPrefix[0] || frame[1] != wantPrefix[1] {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame missing %q prefix", wantPrefix, wantPrefix))
	}
	if frame[memTermOffset] != ';' {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame missing ';' terminator", wantPrefix))
	}

	slot, err := ParseSlot(string(frame[memSlotOffset : memSlotOffset+3]))
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: invalid slot field (P1)", wantPrefix))
	}

	freqField := frame[memFreqOffset : memFreqOffset+memFreqDigits]
	if !allDigits(freqField) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: frequency field (P2) must be 9 digits", wantPrefix))
	}
	freq, err := strconv.ParseUint(string(freqField), 10, 32)
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: frequency field (P2) out of range", wantPrefix))
	}

	sign := frame[memClarSignOffset]
	if sign != '+' && sign != '-' {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: clarifier sign (P3) must be '+' or '-'", wantPrefix))
	}
	clarField := frame[memClarMagOffset : memClarMagOffset+memClarMagDigits]
	if !allDigits(clarField) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: clarifier field (P3) must be 4 digits", wantPrefix))
	}
	clarMag, err := strconv.ParseUint(string(clarField), 10, 16)
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: clarifier field (P3) out of range", wantPrefix))
	}
	clar := int16(clarMag)
	if sign == '-' {
		clar = -clar
	}
	if !validClarHz(clar) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: clarifier (P3) must be a multiple of 10 Hz, magnitude <= 9990", wantPrefix))
	}

	rxClar, err := parseBoolDigit(frame[memRxClarOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: RX CLAR field (P4) must be '0' or '1'", wantPrefix))
	}
	txClar, err := parseBoolDigit(frame[memTxClarOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: TX CLAR field (P5) must be '0' or '1'", wantPrefix))
	}

	mode, err := ParseMode(frame[memModeOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: mode field (P6) invalid", wantPrefix))
	}

	kind := frame[memKindOffset]
	if !validKindByte(kind) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: kind field (P7) must be one of '0','1','2','3','4','5'", wantPrefix))
	}

	ctcss, err := ParseCTCSSState(frame[memCTCSSOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: CTCSS field (P8) invalid", wantPrefix))
	}

	if string(frame[memP9Offset:memP9Offset+2]) != "00" {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: P9 field must be fixed \"00\"", wantPrefix))
	}

	shift, err := ParseShift(frame[memShiftOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: shift field (P10) invalid", wantPrefix))
	}

	return MemoryData{
		Slot:   slot,
		FreqHz: uint32(freq),
		ClarHz: clar,
		RxClar: rxClar,
		TxClar: txClar,
		Mode:   mode,
		Kind:   kind,
		CTCSS:  ctcss,
		Shift:  shift,
	}, nil
}
