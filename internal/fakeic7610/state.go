// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7610

import "fmt"

// The CI-V addresses this package knows. MANUAL-EVIDENCED, from the guide's
// "About the data format" page: 0x98 is the transceiver's default address and
// 0xE0 the controller's.
//
// AddrBroadcast (0x00) is ASSUMED, not evidenced: it is the `to` byte this
// package puts on an unsolicited transceive frame. The document prints NO
// broadcast frame — the only answer-direction skeleton it prints has `to` =
// 0xE0 — so this value is asserted by the flood option so that a consumer can
// be tested against the assumption, and asserting it is not evidence for it.
// See doc.go, "Two floods", and PROVENANCE.md.
const (
	AddrRadio      byte = 0x98
	AddrController byte = 0xE0
	AddrBroadcast  byte = 0x00
)

// The framing bytes. MANUAL-EVIDENCED from the same page.
//
// preambleByte is not exported: a caller never needs to write one, because a
// caller writes whole frames, and exporting it would invite a caller to build
// half a frame.
const (
	preambleByte   byte = 0xFE
	terminatorByte byte = 0xFD
)

// CodeOK and CodeNG are the guide's own fixed reply codes, printed as
// "OK code (Fixed)" and "NG code (Fixed)". MANUAL-EVIDENCED.
//
// Every refusal this fake makes is a CodeNG frame; there is no other kind, and
// there is deliberately no richer error surface. A real CI-V radio has none
// either, and inventing one would let a consumer come to depend on a
// distinction no radio makes.
const (
	CodeOK byte = 0xFB
	CodeNG byte = 0xFA
)

// The commands this package recognises at all. Each is MANUAL-EVIDENCED as a
// wire form; what any of them ANSWERS is a separate question, graded in doc.go
// command by command.
const (
	cnMemory byte = 0x1A // with scMemory: the memory record
	cnID     byte = 0x19 // with scID: the transceiver-ID answer
	cnClear  byte = 0x0B // "Memory clear" — REFUSED, deliberately
	cnPower  byte = 0x18 // 18 01 is power ON — REFUSED, deliberately
	scMemory byte = 0x00
	scMenu   byte = 0x05 // 1A 05, the menu surface this tier does not ship
	scID     byte = 0x00
)

// clearRecordByte is the single data byte the page prints for the clear form
// "1A 00 <hi> <lo> FF" (index ③: "FF"). This fake matches it in order to
// REFUSE it explicitly rather than let it fall through the record-length check,
// so that the refusal is a decision at a named place and not an accident of
// arithmetic. See doc.go, "Deliberate divergences".
const clearRecordByte byte = 0xFF

// RecordLen is the one accepted length, in bytes, of the record that follows a
// 1A 00 channel selector: 25.
//
// DERIVED, not read. It is the sum of the width_bytes column of the D1 rows of
// core/civ/ic7610/testdata/ic7610-transcription-b.csv — 2, 1, 5, 2, 1, 3, 3,
// 10, totalling 27 — less the two selector bytes that CSV counts as its first
// field (①, ②, "Memory channel numbers"). 27 - 2 = 25. doc.go sets out the
// arithmetic in full, including the drawn-cell disagreement the two artefacts
// raise and which this package does not resolve.
//
// A set whose record is any other length is refused with CodeNG. A read always
// answers at this length. WithRecordLength overrides it.
const RecordLen = 25

// NameLen is the width of the memory name field, in bytes: ten. This is the
// ⑱ ~ ㉗ row's width_bytes, and the page's own "Up to 10 characters."
// MANUAL-EVIDENCED.
//
// NamePad is the byte this package treats as a memory name's padding: ASCII
// space, 0x20. ASSUMED, and weakly: the printed character tables have no row
// for a space at all, while the same block's footnote lists "(space)" among the
// usable characters. The two printed things disagree and this package does not
// resolve them.
//
// NOTHING IN THIS PACKAGE WRITES EITHER OF THEM. This fake seeds no channel at
// construction and invents no record contents, so no default record exists for
// a pad byte to appear in; a record's bytes are whatever a consumer set or
// wrote. They are exported because a consumer building a record needs the two
// numbers and should get them from a place that states their grade.
const (
	NameLen      = 10
	NamePad byte = 0x20
)

// ChanP1 and ChanP2 are the two programmed scan edges, whose selectors the page
// prints as 01 00 and 01 01. MANUAL-EVIDENCED as selectors.
//
// They are NEGATIVE on purpose. The scan edges have no channel number, and the
// memory channels run 1..99; a negative sentinel means no arithmetic on a
// memory channel can land on a scan edge by accident, and a caller that passes
// a raw 100 or 0 gets a loud refusal rather than a scan edge it did not ask
// for.
const (
	ChanP1 = -1
	ChanP2 = -2
)

// MemState is one memory record, in wire order: exactly the bytes that follow
// the two channel selector bytes in a 1A 00 frame.
//
// Raw is UNINTERPRETED. This package parses no field of it, knows nothing of
// what any byte means, and will hand back whatever it was given. The field
// layout that gives the record its length lives in doc.go and in
// PROVENANCE.md, as arithmetic over an evidence artefact, and nowhere in this
// package's code.
type MemState struct {
	Raw []byte
}

// clone returns an independent copy, so that a caller mutating the slice it
// handed to SetSlot cannot reach inside the radio, and a caller mutating what
// SlotState returned cannot either.
func (m MemState) clone() MemState {
	if m.Raw == nil {
		return MemState{}
	}
	return MemState{Raw: append([]byte(nil), m.Raw...)}
}

// selectorFor returns the two channel-selector bytes for ch, and whether ch is
// addressable at all.
//
// The memory-channel low byte is built AS PRINTED — one decimal digit per
// nibble, so channel 99 is 0x99 — because the page prints "00 01 ~ 00 99" and
// states no numeric encoding. See doc.go, "Channel selectors".
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

// channelFor is selectorFor's inverse: it decodes a selector pair off the wire
// into a channel, and reports whether the pair addresses anything at all.
// Anything outside the three printed forms addresses nothing.
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

// SetSlot seeds one channel with a record, as though it had been written over
// the wire. ch is a memory channel 1..99, ChanP1 or ChanP2.
//
// It PANICS on a channel this radio cannot address, and on a record whose
// length is not this radio's record length. Both are programming errors in a
// test, and both would otherwise surface several layers away as a puzzling NG
// or a truncated answer; a fake that quietly accepted them would be lying to
// the test that trusted it. The same reasoning internal/fakedx101's fixture
// constructors use.
//
// A record containing 0xFD or 0xFE will truncate or resynchronise the frame
// that carries it — see doc.go, "Framing". This is not checked, because a
// consumer may legitimately want to watch that happen.
func (r *Radio) SetSlot(ch int, m MemState) {
	if _, _, ok := selectorFor(ch); !ok {
		panic(fmt.Sprintf("fakeic7610: channel %d is not addressable — the page prints memory channels 1..99 (selectors 00 01 .. 00 99) and the two scan edges ChanP1 (01 00) and ChanP2 (01 01), and nothing else", ch))
	}
	if len(m.Raw) != r.recordLen {
		panic(fmt.Sprintf("fakeic7610: record for channel %d is %d bytes, want %d — one record has one accepted length (see doc.go, \"Record length\"; WithRecordLength changes it)", ch, len(m.Raw), r.recordLen))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slots[ch] = m.clone()
}

// SlotState returns the record stored for ch, and whether that channel is set
// at all. An unaddressable ch reports not-set rather than panicking: asking
// about a channel is a question, and a question about a channel that cannot
// exist has an answer.
func (r *Radio) SlotState(ch int) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.slots[ch]
	if !ok {
		return MemState{}, false
	}
	return m.clone(), true
}

// ClearSlot makes ch unset, so that a read of it answers CodeNG.
//
// This is the Go-side control the wire deliberately does not offer: both clear
// forms the page prints (1A 00 <hi> <lo> FF, and 0B) are REFUSED on this
// radio's wire, on purpose, so that no code path can clear a channel by sending
// a frame. A test that needs a channel empty says so here, in Go, where the
// statement is visible in the test rather than buried in a byte sequence.
func (r *Radio) ClearSlot(ch int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.slots, ch)
}

// CommandLog returns every (cn, sc) pair this radio has SEEN, in arrival order.
//
// "Seen" means: carried by a well-formed frame addressed to this radio. A frame
// addressed elsewhere is not seen, which is the point — see doc.go, "Framing".
// Refused commands ARE seen and ARE logged: refusing is a thing the radio did,
// and a consumer proving that it never sends a clear needs the log to show the
// absence.
//
// A frame carrying only a cn and no sc (0B is the one this package recognises)
// is logged with a zero sc. That is an ambiguity — 0B and a hypothetical 0B 00
// would log alike — and it is accepted rather than papered over, because the
// alternative is a wider type for a distinction no command in this package's
// surface actually needs.
func (r *Radio) CommandLog() [][2]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][2]byte(nil), r.commandLog...)
}

// BytesWritten returns every byte the host has written to Port(), in order,
// exactly as received.
//
// EVERY byte: before framing, before the address filter, before any parsing.
// Line noise is here, a frame addressed to another radio is here, a malformed
// fragment is here. It is the record of what went down the wire, which is a
// different question from what the radio made of it (CommandLog).
func (r *Radio) BytesWritten() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.bytesWritten...)
}
