// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

import "fmt"

// Half is the record's P1 byte, and it is OVERLOADED BY SLOT CLASS — which is
// why it is not called "split".
//
// On an ordinary memory channel P1 chooses between the simplex/receive
// frequency and the transmit frequency of a split channel (590:1441-1447). On
// a section-defined channel, 100-109, the same byte chooses between the START
// and the END frequency (590:1449-1451, 590:1529-1531). One byte, two
// meanings, and this fake stores the two halves as two records because that
// is what the two frames are.
//
// The values ARE the wire bytes, so a record key needs no translation table.
// The zero value is not one of them.
type Half byte

// The two halves, plus the refusing default.
const (
	HalfUnset Half = 0
	// HalfRXOrStart is P1 '0': "0: Simplex" — the simplex channel's data,
	// the receive frequency of a split channel (590:1441-1445), or the
	// START frequency of a section-defined channel (590:1449-1450).
	HalfRXOrStart Half = '0'
	// HalfTXOrEnd is P1 '1': "1: Split" — the transmit frequency of a split
	// channel (590:1443-1447), or the END frequency of a section-defined
	// channel (590:1450-1451).
	HalfTXOrEnd Half = '1'
)

// valid reports whether h is one of the two printed values.
func (h Half) valid() bool { return h == HalfRXOrStart || h == HalfTXOrEnd }

// recordKey addresses one stored record: a channel number and which of its
// two frames it is.
type recordKey struct {
	channel int
	half    Half
}

// String renders a key for test failures and for the map-walking tests.
func (k recordKey) String() string {
	return fmt.Sprintf("channel %d, P1 %q", k.channel, byte(k.half))
}

// MemState is this fake's OWN in-memory representation of ONE FRAME of one
// memory channel — one record, not one channel: a split channel and a
// section-defined channel each hold two of these (see Half).
//
// It is deliberately NOT core/codeplug.Channel and NOT any core/kw wire type
// — fakets590 must not import either (see doc.go, THE HARD RULE) — so every
// field is stored in the closest thing to raw wire form: single ASCII bytes
// and fixed-width digit strings, matching the 50-position charts in parser.go
// byte for byte (590:1440-1461 for MR's answer, 590:1518-1536 for MW's Set,
// which number the same fifty positions the same way). Building a reply is
// then plain concatenation, with no numeric or text conversion anywhere, and
// no opportunity for this fake to "fix up" a value a real radio would echo
// back as it stands.
//
// THREE PRINTED CONSTANTS ARE ABSENT AND THAT IS DELIBERATE: P10 ("000:
// Always 000", 590:1473), P12 ("0: Always 0", 590:1480) and P13
// ("000000000: Always 000000000", 590:1482) carry no channel information on
// either radio, so there is nothing to store. The answer builder emits them
// and the Set validator requires them; a test that needs a record answering
// something else there reaches it through the frame, not through this struct,
// because a field would invite an image to carry one by accident.
type MemState struct {
	// Freq is P4: the 11-digit ASCII frequency field in Hz, zero-padded
	// (590:1455-1456). Positions 7-17.
	Freq string
	// Mode is P5, the mode nibble as one ASCII byte, position 18. The
	// legend is MD's, referred to by both charts (590:1458): 0..9, with 0
	// and 8 printed "None (setting failure)" (590:1353, 590:1362).
	Mode byte
	// DataMode is P6, position 19, the DA flag "0: DATA mode OFF /
	// 1: DATA mode ON" (590:447-449, referred to at 590:1461-1462).
	DataMode byte
	// ToneMode is P7, position 20: "0: TONE/CTCSS OFF, 1: TONE ON,
	// 2: CTCSS ON, 3: Cross Tone ON" (590:1464-1467). FOUR values, where
	// the TS-480 prints three (480:964) — one of the axes that keeps the
	// two Kenwood fakes apart.
	ToneMode byte
	// ToneNo is P8, positions 21-22: the two-digit index into the TN chart
	// (590:1469, the chart at 590:2296-2306, "00 ~ 42" at 590:2291).
	//
	// STORED, NOT RANGE-CHECKED — doc.go's register entry TONE INDICES ARE
	// STORED, NOT RANGE-CHECKED.
	ToneNo string
	// CTCSSNo is P9, positions 23-24: the two-digit index into the CN chart
	// (590:1471, the chart at 590:416-426, "00 ~ 41" at 590:411). The two
	// charts are NOT the same length, which is why they are two fields and
	// two citations.
	CTCSSNo string
	// Filter is P11, position 28: "0: FILTER A / 1: FILTER B"
	// (590:1476-1477). THE ONE AXIS THE TWO ROWS DIFFER ON — the book
	// qualifies it "In firmware version 1.xx of TS-590S, always '0'."
	// (590:1478) — and this fake accepts and answers both values on both
	// rows, because deciding what a given firmware may carry is the
	// driver's refusal, not the radio's wire grammar (doc.go's register
	// entry BYTE 28 IS ACCEPTED EITHER WAY ON BOTH ROWS).
	Filter byte
	// FMNarrow is P14, positions 39-40: "00: FM Normal / 01: FM Narrow"
	// (590:1484-1485). Two bytes, and its printed meanings cover FM alone —
	// what it means in any other mode is unprinted, which is the driver's
	// A23 and not this fake's business.
	FMNarrow string
	// Lockout is P15, position 41: "0: Channel Lockout OFF / 1: Channel
	// Lockout ON" (590:1487-1488). The TS-480 spends this byte on a printed
	// constant and carries its lockout at position 19 instead, which is why
	// no Kenwood fake may share another's field map.
	Lockout byte
	// Name is P16, positions 42-49: the memory name, "up to 8 digits"
	// (590:1490), STORED VERBATIM AS THE EIGHT BYTES OF THE FIELD.
	//
	// No trimming and no re-padding happen anywhere in this package, and
	// that is the point: how a name shorter than eight characters is padded
	// is A1, an assumption the CODEC carries, and a fake that trimmed on
	// read and padded on write would apply A1 on both sides of every
	// round-trip test and could never contradict it. What arrives is what
	// is stored and what is answered.
	Name string
}

// ChannelState returns this fake's current stored record for one half of one
// channel and whether any record has ever been stored there. A false second
// return is exactly what makes an MR of that half answer the EMPTY record
// (parser.go's handleMR, and the documented empty-channel sentence at
// 590:1492-1493).
//
// It is a test-inspection API: production code (the transport engine, the
// driver) talks to the fake only through Port(), never this method.
func (r *Radio) ChannelState(channel int, half Half) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.records[recordKey{channel: channel, half: half}]
	return s, ok
}

// CurrentChannel returns this fake's "selected channel" state, as last set by
// an MC Set (590:1332-1335), or the construction-time value if no MC Set has
// happened yet (doc.go's register entry THE SELECTED CHANNEL AT
// CONSTRUCTION). An MW never changes it — doc.go's register entry A SET DOES
// NOT MOVE THE SELECTED CHANNEL, pinned by
// TestMW_DoesNotMoveTheSelectedChannel.
func (r *Radio) CurrentChannel() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentChannel
}
