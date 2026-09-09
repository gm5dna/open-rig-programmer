// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

// MemState is this fake's OWN in-memory representation of ONE memory channel:
// the thirteen parameters of the MA0 grid, and nothing else.
//
// ONE RECORD IS ONE CHANNEL HERE, which is the first structural difference
// from internal/fakets590. That family's MR/MW frames carry a P1 selector and
// a split channel is TWO 50-byte records; this radio's MA0 grid carries the
// receive side and the split side in ONE frame — P2 to P7 for the first and P8
// to P10 for the second (890:3171-3200) — so there is no half, no record key
// and no pairing.
//
// It is deliberately NOT core/codeplug.Channel and NOT any core/kw or
// core/kw/ma wire type — fakets890 must not import either (see doc.go, THE
// HARD RULE) — so every field is stored in the closest thing to raw wire form:
// single ASCII bytes and fixed-width strings, matching the position charts in
// parser.go byte for byte (890:3166-3182 for the Set, 890:3189-3204 for the
// Answer, which number the same positions the same way). Building a reply is
// then plain concatenation, with no numeric or text conversion anywhere, and
// no opportunity for this fake to "fix up" a value a real radio would echo
// back as it stands.
//
// THERE IS NO PRINTED CONSTANT IN THIS GRID, and that is a real difference
// from the 590 pair, whose P10, P12 and P13 are hard-wired runs the answer
// builder emits and the Set validator requires. The 890S has none: every one
// of the thirteen parameters is a live field, which is why every one of them
// is a struct member here.
type MemState struct {
	// Freq is P2: the 11-digit ASCII frequency field in Hz, "Blank digits
	// must be entered as '0'" (890:3171-3172). Positions 7-17.
	Freq string
	// Mode is P3, position 18: one nibble of the OM P2 legend the chart
	// refers to (890:3174-3175, the legend at 890:3976-3992). SIXTEEN
	// values, 0-9 and A-F, where the 590 pair's MD legend prints ten.
	Mode byte
	// FMNarrow is P4, position 19: "0: Normal / 1: Narrow"
	// (890:3176-3178).
	FMNarrow byte
	// ToneType is P5, position 20: "0: OFF / 1: Tone / 2: CTCSS /
	// 3: Cross Tone" (890:3180-3185).
	ToneType byte
	// ToneNo is P6, positions 21-22: the two-digit index into the TN chart
	// (890:3186-3187, the chart at 890:5149-5163, "00 ~ 50" with 99 a
	// set-only default).
	//
	// STORED, NOT RANGE-CHECKED — doc.go's register entry TONE INDICES ARE
	// STORED, NOT RANGE-CHECKED.
	ToneNo string
	// CTCSSNo is P7, positions 23-24: the two-digit index into the CN chart
	// (890:3188-3190, the chart at 890:1354-1369, "00 ~ 49"). The two charts
	// are NOT the same length, which is why they are two fields and two
	// citations. Erratum E5 records that CN's chart body heads its index
	// columns P2 where the command's only parameter is P1; nothing here
	// depends on the header, only on the indices.
	CTCSSNo string
	// SplitFreq is P8, positions 25-35: "Split transmission frequency
	// information (11 digits)" (890:3191-3192).
	SplitFreq string
	// SplitMode is P9, position 36: the same OM legend as P3
	// (890:3193-3195).
	SplitMode byte
	// SplitFMNarrow is P10, position 37: "0: Normal / 1: Narrow"
	// (890:3197-3200).
	//
	// The book instructs the HOST to keep it equal to P4 on a split channel
	// (890:3219-3221) and says nothing about what the radio does with a
	// disagreeing Set — doc.go's register entry THE P4/P10 AGREEMENT IS NOT
	// ENFORCED.
	SplitFMNarrow byte
	// Split is P11, position 38: "0: Simplex / 1: Split" (890:3201-3203).
	Split byte
	// Lockout is P12, position 39: "0: Lockout OFF / 1: Lockout ON"
	// (890:3205-3207). The 990S spends the same datum on 1/2 instead
	// (erratum E8's neighbourhood), which is why no MA-family fake may share
	// another's field map.
	Lockout byte
	// Name is P13, position 40 onwards: the channel name, "Up to 10
	// characters" (890:3208-3209), STORED VERBATIM including any trailing
	// space.
	//
	// NO PADDING AND NO TRIMMING HAPPEN ANYWHERE IN THIS PACKAGE, and on
	// this row that is not merely a fake's caution — it is what the frame
	// itself says. The terminator FLOATS after the name (890:3181-3182), so
	// "...AB ;" and "...AB;" are DISTINCT, unambiguous frames and a trailing
	// space is content rather than padding. The codec carries P13 verbatim
	// in both directions for the same reason (the C-MED-1 reversal), and a
	// fake that trimmed on read and padded on write would apply an
	// assumption on both sides of every round trip and could never
	// contradict it.
	Name string
}

// ChannelState returns this fake's current stored record for one channel and
// whether any record has ever been stored there. A false second return is
// exactly what makes an MA0 read of that channel answer the BLANK frame
// (parser.go's handleMA0, and the documented blank-channel sentence at
// 890:3215-3216).
//
// It is a test-inspection API: production code (the transport engine, the
// driver) talks to the fake only through Port(), never this method.
func (r *Radio) ChannelState(channel int) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.records[channel]
	return s, ok
}
