// SPDX-License-Identifier: GPL-3.0-or-later

package fakets2000

import "fmt"

// Half is the record's P1 byte, overloaded by channel number exactly as on
// the TS-480/TS-590: "0: RX frequency, 1: TX frequency" on an ordinary
// channel (ts2000:10692-10693, 10762-10763), and "Memory channel 290 ~ 299:
// P1=0 (start frequency), P1=1 (end frequency)" on the section-defined block
// (ts2000:10726-10727). One byte, two meanings; this fake stores the two
// halves as two records because that is what the two frames are.
type Half byte

// The two halves, plus the refusing default.
const (
	HalfUnset Half = 0
	// HalfRXOrStart is P1 '0'.
	HalfRXOrStart Half = '0'
	// HalfTXOrEnd is P1 '1'.
	HalfTXOrEnd Half = '1'
)

func (h Half) valid() bool { return h == HalfRXOrStart || h == HalfTXOrEnd }

// recordKey addresses one stored record: a channel number (0-299) and which
// of its two frames it is.
type recordKey struct {
	channel int
	half    Half
}

func (k recordKey) String() string {
	return fmt.Sprintf("channel %d, P1 %q", k.channel, byte(k.half))
}

// MemState is this fake's OWN in-memory representation of ONE FRAME of one
// memory channel. It is deliberately NOT core/codeplug.Channel and NOT any
// core/kw wire type (doc.go, THE HARD RULE): every field is stored in the
// closest thing to raw wire form, matching the 50-position chart in
// parser.go byte for byte (ts2000:10692-10727), so a reply is plain
// concatenation with no numeric conversion and no "fixing up" of a value a
// real radio would echo back as it stands.
//
// EVERY ONE OF THE SIXTEEN P-FIELDS IS LIVE ON THIS ROW — unlike the
// TS-480/TS-590 grid, there is no printed-fixed byte anywhere in this record
// (matrix §2's whole point) — so MemState carries all sixteen, where the
// siblings' structs omit the ones their own books print as constants.
type MemState struct {
	// Freq is P4: the 11-digit ASCII frequency field in Hz, zero-padded
	// (ts2000:10696-10697). Positions 7-17.
	Freq string
	// Mode is P5, the mode nibble as one ASCII byte, position 18. Legend:
	// MD, 1-9 with 8 a printed hole (matrix §5) and 0 undocumented — stored
	// regardless, doc.go's register entry 9.
	Mode byte
	// Lockout is P6, position 19: "0: Lockout OFF, 1: Lockout ON."
	// (ts2000:10700-10701).
	Lockout byte
	// ToneMode is P7, position 20: "0: OFF, 1: TONE, 2: CTCSS, 3: DCS."
	// (ts2000:10702-10703). Four values, all printed.
	ToneMode byte
	// ToneNo is P8, positions 21-22: the two-digit index "See page 35"
	// (ts2000:10704-10705). STORED, NOT RANGE-CHECKED — doc.go's register
	// entry 7.
	ToneNo string
	// CTCSSNo is P9, positions 23-24: the two-digit CTCSS index, "See CN
	// command" (ts2000:10706-10707). STORED, NOT RANGE-CHECKED.
	CTCSSNo string
	// DCSCode is P10, positions 25-27: the three-digit DCS index, "See QC
	// command" (ts2000:10709-10710) — LIVE on this row where the 590/480
	// grid prints "000". STORED, NOT RANGE-CHECKED — doc.go's register
	// entry 6.
	DCSCode string
	// Reverse is P11, position 28: "REVERSE status." (ts2000:10712), no
	// legend printed anywhere. STORED AS A SINGLE DIGIT, NOT RANGE-CHECKED —
	// doc.go's register entry 5.
	Reverse byte
	// Shift is P12, position 29: "0: Simplex / 1: + / 2: - / 3: = (All
	// E-types)" (ts2000:10935-10938 per the matrix's own citation) — LIVE on
	// this row where the 590/480 grid prints "0".
	Shift byte
	// Offset is P13, positions 30-38: the nine-digit BCD offset frequency,
	// "See OS command" (ts2000:10716-10717) — LIVE where the 590/480 grid
	// prints "000000000". STORED, NOT RANGE-CHECKED — doc.go's register
	// entry 6.
	Offset string
	// Step is P14, positions 39-40: "Step size. See ST command."
	// (ts2000:10720-10721). STORED, NOT RANGE-CHECKED — doc.go's register
	// entry 8.
	Step string
	// MemoryGroup is P15, position 41: "Memory Group number (0 ~ 9)."
	// (ts2000:10722-10723) — LIVE where the 590/480 grid prints a fixed
	// lockout/zero byte. Every one of the ten legend values is admitted; the
	// legend itself is the whole of the ten-value range.
	MemoryGroup byte
	// Name is P16, positions 42-49: the memory name, "A maximum of 8
	// characters." (ts2000:10723-10724), STORED VERBATIM AS THE EIGHT BYTES
	// OF THE FIELD, exactly like every sibling Kenwood fake's P16.
	Name string
}

// ChannelState returns this fake's current stored record for one half of one
// channel and whether any record has ever been stored there. A false second
// return is what makes an MR of that half answer the invented empty-channel
// shape (parser.go's handleMR; doc.go's register entry 3).
//
// It is a test-inspection API: production code talks to the fake only
// through Port(), never this method.
func (r *Radio) ChannelState(channel int, half Half) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.records[recordKey{channel: channel, half: half}]
	return s, ok
}

// CurrentChannel returns this fake's "selected channel" state, as last set
// by an MC Set, or the construction-time value if no MC Set has happened yet
// (doc.go's register entry 11). An MW never changes it — doc.go's register
// entry 10.
func (r *Radio) CurrentChannel() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentChannel
}
