// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
)

// recordLen is this radio's own 16-byte record width (matrix §1.2),
// shared by every record the full dump carries (current Operating Data,
// VFO-A, VFO-B, and each of the 113 memories — same table heading).
const recordLen = 16

// Byte offsets within one 16-byte record (matrix §1.2, "16-BYTE DATA
// RECORD STRUCTURE"). This driver decodes its own record shape directly
// rather than through core/bincat.ParseRecord/Record: that type's
// FT-890/900-shaped decode (24-bit binary frequency at a single 3-byte
// offset, a tone byte) does not fit this radio's 4-byte nibble-decimal
// frequency and tone-less record at all.
const (
	offBandSelect = 0 // Band Selection: memory-mask (bit 0x80) + scan-skip (bit 0x40) flags, plus a 28-band code
	offFreq       = 1 // 4 bytes, nibble-decimal (see decodeFreqTensOfHz)
	offClarifier  = 5 // 2 bytes, 16-bit 2's-complement, x0.625 Hz
	offMode       = 7 // 1 byte, bits 5-7 a 3-bit code (recordModeBase)
	offFlags      = 9 // VFO/MEM Operating Flags — see flag bit consts below
)

// Bits within the Band Selection byte (offset 0) — "The Bit 0 and Bit 1
// of the first field are used as flags for the memory mask and scan skip
// feature. A bit value of '1' means enabled" (matrix's own source,
// ft1000mpmarkv_manual layout:4850-4863, printed p.90-91). The manual's
// own Bit-numbering is MSB-first (Bit0 is the byte's 0x80 bit — the same
// convention the Operating Mode and VFO/MEM Operating Flags bytes use,
// confirmed by their own worked examples).
const (
	flagMemMask  byte = 0x80 // "1" = this slot is masked (hidden/blanked)
	flagScanSkip byte = 0x40 // "1" = skipped while scanning
)

// Bits within the VFO/MEM Operating Flags byte (offset 9) — "VFO/MEM
// Indicators - Five flags indicate the status of Clarifier (Rx & Tx),
// Repeater Offset (+/-), and Antenna Selection (A/B/RX)... for all flag
// bits, 1 = On, 0 = Off" (ft1000mpmarkv_manual layout:4929-4946, printed
// p.91, the table headed "IF Filter Selection Byte (8)" that in fact
// describes byte 9's own five flags — a manual mislabelling this driver
// works around rather than repeats). Bits 0-1 are dummy; Bits 2-3 are ANT
// SELECT (unused by this driver); Bit4 = -RPT (Minus), Bit5 = +RPT
// (Plus), Bit6 = RX CLAR, Bit7 = TX CLAR — MSB-first, so Bit4 = 0x08,
// Bit5 = 0x04, Bit6 = 0x02, Bit7 = 0x01.
const (
	flagRptMinus byte = 0x08
	flagRptPlus  byte = 0x04
	flagRxClar   byte = 0x02
	flagTxClar   byte = 0x01
)

// swapNibbles exchanges b's two 4-bit halves.
func swapNibbles(b byte) byte { return b<<4 | b>>4 }

// decodeFreqTensOfHz decodes a record's 4-byte Operating Frequency field
// (offset 1-4) into tens-of-Hz.
//
// RE-DERIVED FROM THE WORKED EXAMPLE, not merely restating the matrix's
// prose (which states the nibble-per-digit RULE but not the byte-level
// algorithm): ft1000mpmarkv_manual layout:4884-4898 gives raw bytes
// 00,05,24,10 (hex) for 14.250.00 MHz = 1,425,000 tens-of-Hz. Swapping
// each byte's nibbles gives 00,50,42,01 — BYTE-IDENTICAL to the
// write-side SetFreq worked example's own packed-BCD bytes for the same
// frequency (bcd_test.go's TestEncodeBCD_FT1000MPWorkedExample) — so the
// read-side field is exactly the write-side's packed BCD with each byte's
// nibbles swapped, decoded via the SAME bincat.DecodeBCD this package's
// write side (write.go) builds frames with. record_test.go pins this
// against the manual's own raw bytes directly, not only against the
// swapped-then-reused-encoder round trip.
func decodeFreqTensOfHz(b [4]byte) (uint64, error) {
	swapped := make([]byte, 4)
	for i, x := range b {
		swapped[i] = swapNibbles(x)
	}
	v, err := bincat.DecodeBCD(swapped)
	if err != nil {
		return 0, fmt.Errorf("%w: frequency field: %w", errRecord, err)
	}
	return v, nil
}

// record is this driver's own decode of one 16-byte record — deliberately
// narrower than bincat.Record: it carries only what ReadChannel needs
// (matrix §4's registered fields), not every byte the manual names.
type record struct {
	Masked   bool // memory-mask flag (offBandSelect, flagMemMask) — this driver's empty-slot signal
	FreqHz   uint64
	ClarHz   int
	RxClar   bool
	TxClar   bool
	ModeByte byte // the record's own 3-bit code (0-6), not a 0CH wire code
	Shift    string
}

// parseRecord decodes raw as one 16-byte record. raw must be exactly
// recordLen bytes.
func parseRecord(raw []byte) (record, error) {
	if len(raw) != recordLen {
		return record{}, fmt.Errorf("%w: record is %d bytes, want %d", errRecord, len(raw), recordLen)
	}

	var rec record
	rec.Masked = raw[offBandSelect]&flagMemMask != 0

	freqTens, err := decodeFreqTensOfHz([4]byte(raw[offFreq : offFreq+4]))
	if err != nil {
		return record{}, err
	}
	rec.FreqHz = freqTens * 10

	clarRaw := int16(uint16(raw[offClarifier])<<8 | uint16(raw[offClarifier+1]))
	rec.ClarHz = int(float64(clarRaw) * 0.625)

	rec.ModeByte = (raw[offMode] >> 5) & 0x07

	flags := raw[offFlags]
	rec.RxClar = flags&flagRxClar != 0
	rec.TxClar = flags&flagTxClar != 0
	minus, plus := flags&flagRptMinus != 0, flags&flagRptPlus != 0
	switch {
	case minus && plus:
		return record{}, fmt.Errorf("%w: operating flags byte %#02x sets both Minus and Plus shift bits", errRecord, flags)
	case minus:
		rec.Shift = "MINUS"
	case plus:
		rec.Shift = "PLUS"
	default:
		rec.Shift = "SIMPLEX"
	}

	return rec, nil
}
