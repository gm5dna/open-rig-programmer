// SPDX-License-Identifier: GPL-3.0-or-later

package fakets870s

import "fmt"

// Half is the record's P1 byte, and it is OVERLOADED BY CHANNEL NUMBER, the
// same shape internal/fakets480's Half and internal/fakets590's Half both
// take for their own P1 bytes.
//
// On channels 00-98, P1 chooses between the receive and the transmit
// frequency: "0: Receive, 1: Transmit" (Format 9, ts870s:8276-8278). On
// channel 99, the SAME byte chooses between the START and the END frequency:
// "P1 must be '0' to read the CH 99 Start frequency and '1' to read the End
// frequency" (ts870s:9105-9106), and the mirrored sentence on the write side
// (ts870s:9162-9163). This fake stores the two halves as two records, because
// that is what the two frames are; it does not special-case channel 99 — the
// byte means what P1 always means, and only the DRIVER'S OWN publication
// choice (matrix §1.4/§2.2: MEM slot "99"'s End-frequency half is left
// unreachable through that programme) restricts which half a real session
// ever sends for that one channel. This fake still serves both, because the
// book prints both.
type Half byte

// The two halves. The zero value is not one of them.
const (
	HalfUnset Half = 0
	// HalfRXOrStart is P1 '0': the receive frequency of channels 00-98
	// (ts870s:8276-8278), or the START frequency of channel 99
	// (ts870s:9105-9106).
	HalfRXOrStart Half = '0'
	// HalfTXOrEnd is P1 '1': the transmit frequency of channels 00-98, or the
	// END frequency of channel 99.
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

// String renders a key for test failures.
func (k recordKey) String() string {
	return fmt.Sprintf("channel %d, P1 %q", k.channel, byte(k.half))
}

// MemState is this fake's OWN in-memory representation of ONE FRAME of one
// memory channel. It is deliberately NOT core/codeplug.Channel and NOT any
// core/kw wire type (see doc.go, THE HARD RULE): every field is stored in
// the closest thing to raw wire form, matching the position table in
// parser.go byte for byte, so building a reply is plain concatenation with
// no numeric conversion anywhere.
//
// UNLIKE THE 480/590 FAMILY, THIS RECORD HAS NO PRINTED-CONSTANT RUN AT ALL.
// P2 and P9 are not filler bytes here — they are ABSENT bytes (doc.go) — so
// there is nothing for MemState to carry for either, and nothing for the
// answer builder to emit or the Set validator to require at those positions.
type MemState struct {
	// Freq is P4: the 11-digit ASCII frequency field in Hz, zero-padded
	// (Format 4, ts870s:8267-8269). Positions 6-16.
	Freq string
	// Mode is P5, position 17: the mode nibble as one ASCII byte, Format 2's
	// ten-value legend (ts870s:8256-8262) — 0 and 8 are both "No mode"/"Tune"
	// holes, admitted and stored like every other digit (see parser.go's
	// validModeByte and doc.go's register entry 3).
	Mode byte
	// Lockout is P6, position 18: "0: Not locked out, 1: Locked out" (Format
	// 10, ts870s:8283-8284 area).
	Lockout byte
	// ToneMode is P7, position 19: "0: OFF, 1: ON" (Format 1, ts870s:8256) —
	// TWO values, narrower than every sibling Kenwood fake's tone-mode byte.
	ToneMode byte
	// ToneNo is P8, positions 20-21: the two-digit index into the 39-entry
	// subtone chart ("01~39", Format 14, ts870s:8296-8299).
	//
	// STORED, NOT RANGE-CHECKED — doc.go's register entry 4.
	ToneNo string
}

// ChannelState returns this fake's current stored record for one half of one
// channel and whether any record has ever been stored there. A false second
// return is what makes an MR of that half answer the ZERO record
// (parser.go's handleMR) — doc.go's register entry 5, AN UNWRITTEN OR VACANT
// CHANNEL'S EITHER HALF ANSWERS THE ZERO RECORD.
//
// It is a test-inspection API: production code talks to the fake only
// through Port(), never this method.
func (r *Radio) ChannelState(channel int, half Half) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.records[recordKey{channel: channel, half: half}]
	return s, ok
}
