// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7800

import "fmt"

// The CI-V addresses this package knows. MANUAL-EVIDENCED, matrix §3.4: 0x6A
// is the IC-7800's default address, 0xE0 the controller's.
//
// AddrBroadcast (0x00) is ASSUMED, not evidenced — matrix §3.5(b), register
// 7800-R9. See doc.go, "Two floods".
const (
	AddrRadio      byte = 0x6A
	AddrController byte = 0xE0
	AddrBroadcast  byte = 0x00
)

// Framing bytes. MANUAL-EVIDENCED, matrix §3.4/§3.10 frame skeleton.
const (
	preambleByte   byte = 0xFE
	terminatorByte byte = 0xFD
)

// CodeOK and CodeNG are the matrix's own fixed reply codes (§3.8(a): "OK
// code (fixed)" / "NG code (fixed)"). MANUAL-EVIDENCED.
const (
	CodeOK byte = 0xFB
	CodeNG byte = 0xFA
)

// The commands this package recognises at all. MANUAL-EVIDENCED as wire
// forms (matrix §3.7, §3.12(i), §3.13); what each ANSWERS is graded in
// doc.go command by command.
const (
	cnMemory byte = 0x1A // with scMemory: the memory record (matrix §3.7)
	cnID     byte = 0x19 // with scID: the transceiver-ID answer (matrix §3.12(i))
	cnClear  byte = 0x0B // "Memory clear" (matrix §3.13) — REFUSED, deliberately
	cnPower  byte = 0x18 // power ON — REFUSED, deliberately; see doc.go
	scMemory byte = 0x00
	scMenu   byte = 0x05 // 1A 05, the menu surface this tier does not ship (matrix §3.16(f))
	scID     byte = 0x00
)

// clearRecordByte is the single data byte the matrix prints for the clear
// form "1A 00 <hi> <lo> FF" (§3.13). Matched explicitly so the refusal is a
// decision at a named place, not a by-product of the length check below.
const clearRecordByte byte = 0xFF

// RecordLen is the one accepted length, in bytes, of the record that follows
// a 1A 00 channel selector: 25. DERIVED from the matrix §3.11 field-width
// table — see doc.go, "Record length". WithRecordLength overrides it.
const RecordLen = 25

// NameLen is the memory-name field's width: 10 bytes (MANUAL-EVIDENCED,
// matrix §1 row 7 / §3.9(i)). NamePad is this package's assumed pad byte for
// a name shorter than NameLen: 0x20 (ASSUMED, matrix §3.9(iv), register
// 7800-R4). NOTHING IN THIS PACKAGE WRITES EITHER — see doc.go.
const (
	NameLen      = 10
	NamePad byte = 0x20
)

// ChanP1 and ChanP2 are the two programmed scan edges, whose selectors the
// matrix gives as 01 00 and 01 01 (§1b). MANUAL-EVIDENCED as selectors.
// Negative on purpose: memory channels run 1..99, so no arithmetic on one
// can land on a scan edge by accident.
const (
	ChanP1 = -1
	ChanP2 = -2
)

// MemState is one memory record, in wire order: exactly the bytes that
// follow the two channel selector bytes in a 1A 00 frame.
//
// Raw is UNINTERPRETED. This package parses no field of it — see doc.go,
// "Record length", on the byte-8 tone_mode/data_mode nibble swap this radio
// carries against the IC-7610 family, which this package never looks at.
type MemState struct {
	Raw []byte
}

func (m MemState) clone() MemState {
	if m.Raw == nil {
		return MemState{}
	}
	return MemState{Raw: append([]byte(nil), m.Raw...)}
}

// selectorFor returns the two channel-selector bytes for ch, and whether ch
// is addressable at all.
//
// Built AS PRINTED — one decimal digit per nibble, so channel 99 is 0x99 —
// matching the matrix transcription's own encoding label ("bcd_packed") for
// field "q,w". See doc.go, "Channel selectors".
func selectorFor(ch int) (hi, lo byte, ok bool) {
	switch ch {
	case ChanP1:
		return 0x01, 0x00, true
	case ChanP2:
		return 0x01, 0x01, true
	}
	if ch < 1 || ch > 99 {
		return 0, 0, false
	}
	return 0x00, byte((ch/10)<<4 | (ch % 10)), true
}

// channelFor is selectorFor's inverse: decodes a selector pair off the wire
// into a channel, and reports whether the pair addresses anything at all.
func channelFor(hi, lo byte) (int, bool) {
	switch {
	case hi == 0x01 && lo == 0x00:
		return ChanP1, true
	case hi == 0x01 && lo == 0x01:
		return ChanP2, true
	case hi == 0x00:
		tens, units := int(lo>>4), int(lo&0x0F)
		if tens > 9 || units > 9 {
			return 0, false
		}
		n := tens*10 + units
		if n < 1 || n > 99 {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// SetSlot seeds one channel with a record, as though it had been written
// over the wire. ch is a memory channel 1..99, ChanP1 or ChanP2.
//
// PANICS on a channel this radio cannot address, and on a record whose
// length is not this radio's record length — both programming errors in a
// test that would otherwise surface several layers away as a puzzling NG.
func (r *Radio) SetSlot(ch int, m MemState) {
	if _, _, ok := selectorFor(ch); !ok {
		panic(fmt.Sprintf("fakeic7800: channel %d is not addressable — the matrix gives memory channels 1..99 (selectors 00 01 .. 00 99) and the two scan edges ChanP1 (01 00) and ChanP2 (01 01), and nothing else", ch))
	}
	if len(m.Raw) != r.recordLen {
		panic(fmt.Sprintf("fakeic7800: record for channel %d is %d bytes, want %d — one record has one accepted length (see doc.go, \"Record length\"; WithRecordLength changes it)", ch, len(m.Raw), r.recordLen))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slots[ch] = m.clone()
}

// SlotState returns the record stored for ch, and whether that channel is
// set at all. An unaddressable ch reports not-set rather than panicking.
func (r *Radio) SlotState(ch int) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.slots[ch]
	if !ok {
		return MemState{}, false
	}
	return m.clone(), true
}

// ClearSlot makes ch unset, so a read of it answers per the radio's
// empty-channel mode (CodeNG by default, or an all-FF record under
// WithAllFFEmpty). The Go-side control the wire deliberately does not offer
// — both printed clear forms are refused on the wire (doc.go).
func (r *Radio) ClearSlot(ch int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.slots, ch)
}

// CommandLog returns every (cn, sc) pair this radio has SEEN, in arrival
// order — carried by a well-formed frame addressed to this radio. A frame
// addressed elsewhere is not seen. Refused commands ARE seen and ARE logged.
func (r *Radio) CommandLog() [][2]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][2]byte(nil), r.commandLog...)
}

// BytesWritten returns every byte the host has written to Port(), in order,
// exactly as received — before framing, before the address filter, before
// any parsing.
func (r *Radio) BytesWritten() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.bytesWritten...)
}
