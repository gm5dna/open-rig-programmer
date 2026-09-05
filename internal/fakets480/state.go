// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

import "fmt"

// Half is the record's P1 byte, and it is OVERLOADED BY CHANNEL NUMBER —
// which is why it is not called "split".
//
// On any channel P1 chooses between the receive and the transmit frequency:
// "0: RX frequency, 1: TX frequency" (480:908, 480:951). On channels 90 to
// 99 the same byte chooses between the START and the END frequency: "Memory
// channel 90 ~ 99: P1=0 (start frequency), P1=1 (end frequency)"
// (480:943-944, 480:986-987). One byte, two meanings, and this fake stores
// the two halves as two records because that is what the two frames are.
//
// THE UPPER HALF IS NOT A PUBLISHED SLOT ON THIS ROW. Decision 15 gives the
// TS-480 one flat MEM bank, 00-99, and no scan bank — there is no bank field
// in this radio's record at all (480:827) — so nothing this programme sends
// ever carries P1=1 on this row. The fake still SERVES the frame, because the
// book prints it; it simply ships no image for it (image.go).
//
// The values ARE the wire bytes, so a record key needs no translation table.
// The zero value is not one of them.
type Half byte

// The two halves, plus the refusing default.
const (
	HalfUnset Half = 0
	// HalfRXOrStart is P1 '0': the receive frequency of any channel
	// (480:908), or the START frequency of channels 90-99 (480:943-944).
	HalfRXOrStart Half = '0'
	// HalfTXOrEnd is P1 '1': the transmit frequency of any channel
	// (480:908), or the END frequency of channels 90-99 (480:943-944).
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
// memory channel — one record, not one channel: channels 90-99 each hold two
// of these, and so does any channel a test writes both halves of (see Half).
//
// It is deliberately NOT core/codeplug.Channel and NOT any core/kw wire type
// — fakets480 must not import either (see doc.go, THE HARD RULE) — so every
// field is stored in the closest thing to raw wire form: single ASCII bytes
// and fixed-width digit strings, matching the 50-position charts in parser.go
// byte for byte (480:923-943 for MR's answer, 480:955-976 for MW's Set,
// which number the same fifty positions the same way). Building a reply is
// then plain concatenation, with no numeric or text conversion anywhere, and
// no opportunity for this fake to "fix up" a value a real radio would echo
// back as it stands.
//
// SIX PRINTED-CONSTANT RUNS ARE ABSENT AND THAT IS DELIBERATE — three more
// than the 590 pair's record has. P2 ("Always 0 for the TS-480.", 480:910),
// P10 ("Always 000", 480:929), P11 (480:931), P12 (480:933), P13 ("Always
// 000000000", 480:935) and P15 (480:939) carry no channel information, so
// there is nothing to store. The answer builder emits them and the Set
// validator requires them; a test that needs a record answering something
// else there reaches it through the frame, not through this struct, because
// a field would invite an image to carry one by accident.
type MemState struct {
	// Freq is P4: the 11-digit ASCII frequency field in Hz, zero-padded
	// (480:914). Positions 7-17.
	Freq string
	// Mode is P5, the mode nibble as one ASCII byte, position 18. The
	// legend is MD's, referred to by both charts (480:917, 480:959): 0..9,
	// with 0 printed "No mode (Not used for the TS-480)" and 8 "Tune (Not
	// used for the TS-480)" (480:843, 480:853).
	Mode byte
	// Lockout is P6, position 19: "Lockout status. 0: Lockout OFF,
	// 1: Lockout ON." (480:920).
	//
	// THIS IS THE 590 PAIR'S DATA-MODE POSITION. They carry their lockout
	// at position 41 instead, where this radio prints a constant — which is
	// why no Kenwood fake may share another's field map.
	Lockout byte
	// ToneMode is P7, position 20: "0: OFF, 1: TONE, 2: CTCSS"
	// (480:922). THREE values, where the 590 pair print four
	// (590:1464-1467) — this book prints no cross tone at all.
	ToneMode byte
	// ToneNo is P8, positions 21-22: the two-digit index into the TN chart
	// ("00 ~ 42", 480:1557; the table itself is in the instruction manual,
	// 480:1559-1560, not in this document).
	//
	// STORED, NOT RANGE-CHECKED — doc.go's register entry TONE INDICES ARE
	// STORED, NOT RANGE-CHECKED.
	ToneNo string
	// CTCSSNo is P9, positions 23-24: the two-digit index into the CN chart
	// ("00 ~ 41", 480:337). The two charts are NOT the same length, which is
	// why they are two fields and two citations.
	CTCSSNo string
	// Step is P14, positions 39-40: "Step size. Refer to the ST command."
	// (480:937). ST's legend is MODE-CONDITIONAL over two different ranges
	// (480:1494-1500) and this field carries no mode, which is the design's
	// A22 and the reason every TS-480 channel write in this programme is
	// refused.
	//
	// STORED, NOT RANGE-CHECKED — doc.go's register entry THE STEP INDEX IS
	// STORED, NOT RANGE-CHECKED.
	Step string
	// Name is P16, positions 42-49: the memory name, "A maximum of 8
	// characters." (480:941), STORED VERBATIM AS THE EIGHT BYTES OF THE
	// FIELD.
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
// return is what makes an MR of that half answer the ZERO record
// (parser.go's handleMR) — doc.go's register entry AN UNWRITTEN CHANNEL
// ANSWERS THE ZERO RECORD, which is this fake asserting A4.
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
// an MC Set (480:830), or the construction-time value if no MC Set has
// happened yet (doc.go's register entry THE SELECTED CHANNEL AT
// CONSTRUCTION). An MW never changes it — doc.go's register entry A SET DOES
// NOT MOVE THE SELECTED CHANNEL, pinned by
// TestMW_DoesNotMoveTheSelectedChannel.
func (r *Radio) CurrentChannel() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentChannel
}
