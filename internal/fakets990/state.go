// SPDX-License-Identifier: GPL-3.0-or-later

package fakets990

// MemState is this fake's OWN in-memory representation of ONE memory channel:
// the eighteen parameters of the MA0 grid, and nothing else.
//
// ONE RECORD IS ONE CHANNEL HERE, and its two frequency sides are not the
// 890S's. That radio's grid carries a RECEIVE side and a SPLIT TRANSMISSION
// side; this one carries "frequency 1" and "frequency 2" (990:2906, 990:2928),
// with a separate P15 saying whether the channel is split at all (990:2947-2948)
// and a P16 saying whether it receives on both (990:2950-2951). The fields
// below are therefore named for the chart's own words rather than for the
// sibling's, so that a byte cannot be moved between the two radios' models by
// a name that looks familiar.
//
// It is deliberately NOT core/codeplug.Channel and NOT any core/kw or
// core/kw/ma wire type — fakets990 must not import either (see doc.go, THE
// HARD RULE) — so every field is stored in the closest thing to raw wire form:
// single ASCII bytes and fixed-width strings, matching the position charts in
// parser.go byte for byte (990:2893-2915 for the Set, 990:2919-2938 for the
// Answer, which number the same positions the same way). Building a reply is
// then plain concatenation, with no numeric or text conversion anywhere, and
// no opportunity for this fake to "fix up" a value a real radio would echo
// back as it stands.
type MemState struct {
	// Class is P2, position 7: "0: Single Memory channel / 1: Dual Memory
	// channel / 2: Section defined Memory channel" (990:2898-2900).
	//
	// IT IS AN ANSWER-DIRECTION FIELD. On a Set the byte is a dummy the
	// radio discards — "The memory channel type is decided while setting the
	// P9 and P10 values, so this parameter is ignored. Enter a dummy value."
	// (990:2901-2903), the design's A14 — so what a channel ANSWERS here is
	// the radio's own reading of the record. doc.go's register entry THE
	// SET'S P2 IS IGNORED AND THE ANSWER'S CLASS FOLLOWS FREQUENCY 2.
	Class byte
	// Freq is P3, positions 8-18: "Frequency 1 (11 digits in Hz.)"
	// (990:2906).
	Freq string
	// Mode is P4, position 19: one nibble of the OM P2 legend the chart
	// refers to (990:2909-2910, the legend at 990:3707-3730). TWENTY-FOUR
	// values, 0-9 and A-N, where the 890S's prints sixteen.
	Mode byte
	// FMNarrow is P5, position 20: "0: FM Wide for frequency 1 /
	// 1: FM Narrow for frequency 1" (990:2913-2914).
	FMNarrow byte
	// ToneType is P6, position 21: "0: FM Tone function OFF / 1: Tone /
	// 2: CTCSS / 3: Cross Tone", all for frequency 1 (990:2916-2919).
	ToneType byte
	// ToneNo is P7, positions 22-23: the two-digit index into the TN chart
	// (990:2921-2922, the chart at 990:4949-4974, "00 ~ 50" with 99 a
	// set-only default).
	//
	// STORED, NOT RANGE-CHECKED — doc.go's register entry TONE INDICES ARE
	// STORED, NOT RANGE-CHECKED.
	ToneNo string
	// CTCSSNo is P8, positions 24-25: the two-digit index into the CN chart
	// (990:2925-2926, the chart at 990:1240-1267, "00 ~ 49"). The two charts
	// are NOT the same length, which is why they are two fields and two
	// citations.
	CTCSSNo string
	// Freq2 is P9, positions 26-36: "Frequency 2 (11 digits in Hz. Blank
	// digits must be entered as 0.)" (990:2928).
	//
	// It is all zeroes on a single memory channel, which the book prints
	// rather than this fake assuming: "When reading a single memory channel,
	// all parameters for frequency 2 become 0." (990:2964-2965), the design's
	// A16.
	Freq2 string
	// Mode2 is P10, position 37: the same OM legend as P4 (990:2930-2931).
	// The chart's cell says "the P1 value of the OM command" where P1 is that
	// command's Main/Sub band selector and P2 is the mode — erratum E9 — so
	// the legend read here is the same one P4 names.
	Mode2 byte
	// FMNarrow2 is P11, position 38: "0: FM Wide for frequency 2 /
	// 1: FM Narrow for frequency 2" (990:2933-2934).
	//
	// THIS BOOK PRINTS NO INSTRUCTION TO KEEP IT EQUAL TO P5, where the
	// 890S's tells a host to keep its own pair equal on a split channel
	// (890:3219-3221). There is correspondingly less to enforce, and this
	// fake enforces neither — doc.go's register entry SET-DIRECTION FIELD
	// STRICTNESS covers each byte's own legend and nothing across fields.
	FMNarrow2 byte
	// ToneType2 is P12, position 39: the same four values as P6, for
	// frequency 2 (990:2936-2939).
	ToneType2 byte
	// ToneNo2 is P13, positions 40-41: the TN index for frequency 2
	// (990:2941-2942).
	ToneNo2 string
	// CTCSSNo2 is P14, positions 42-43: the CN index for frequency 2
	// (990:2944-2945).
	CTCSSNo2 string
	// Split is P15, position 44: "0: Simplex / 1: Split" (990:2947-2948).
	Split byte
	// DualRX is P16, position 45: "0: Dual reception OFF / 1: Dual reception
	// ON" (990:2950-2951). THE 890S'S GRID HAS NO SUCH FIELD.
	DualRX byte
	// Lockout is P17, position 46: "1: Scan Lockout OFF / 2: Scan Lockout
	// ON" (990:2952-2954).
	//
	// ONE AND TWO, NOT ZERO AND ONE. The 890S spends the same datum on 0/1
	// (890:3205-3207) and this radio's own MA3 prints 0/1 for the same
	// function on the facing page (990:3020-3021) — erratum E8 — which is why
	// no MA-family fake may share another's field map, or even its own book's
	// neighbouring command's.
	Lockout byte
	// Name is P18, positions 47-56: the channel name, "Channel Name (Up to 10
	// digits.)" printed against a FIXED TEN-BYTE WINDOW (990:2955-2956,
	// erratum E13), STORED VERBATIM.
	//
	// NO PADDING AND NO TRIMMING HAPPEN ANYWHERE IN THIS PACKAGE. The window
	// is fixed, so a Set always carries ten bytes and there is nothing for a
	// fake to pad; and the design's A1 — space padding on write, trailing
	// spaces not part of the name on read — is the CODEC's and the DRIVER's
	// rule, on the side that has to turn a window into a name. A fake that
	// applied A1 too would apply it on both sides of every round trip and
	// could never contradict it. doc.go's register entry THE NAME WINDOW IS
	// TEN BYTES, CARRIED VERBATIM.
	Name string
}

// ChannelState returns this fake's current stored record for one channel and
// whether any record has ever been stored there. A false second return is
// exactly what makes an MA0 read of that channel answer the BLANK frame
// (parser.go's handleMA0, and the documented blank-channel sentence at
// 990:2962-2963).
//
// It is a test-inspection API: production code (the transport engine, the
// driver) talks to the fake only through Port(), never this method.
func (r *Radio) ChannelState(channel int) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.records[channel]
	return s, ok
}
