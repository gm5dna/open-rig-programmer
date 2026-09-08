// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"strings"
	"testing"
)

// frame890 is a well-formed TS-890S MA0 answer for channel 007 with the name
// window carrying name: 39 printed positions, then P13, then the floating
// terminator (890:3187-3204).
func frame890(name string) string {
	return "MA0" + // 1-3
		"007" + // P1, 4-6
		"00014250000" + // P2, 7-17
		"2" + // P3, 18
		"0" + // P4, 19
		"0" + // P5, 20
		"00" + // P6, 21-22
		"00" + // P7, 23-24
		"00000000000" + // P8, 25-35
		"0" + // P9, 36
		"0" + // P10, 37
		"0" + // P11, 38
		"0" + // P12, 39
		name + ";" // P13, 40 ~, then ';' at x
}

// with890 returns frame890's shape with the byte at 1-based position pos
// replaced by b.
func with890(name string, pos int, b string) string {
	f := frame890(name)
	return f[:pos-1] + b + f[pos-1+len(b):]
}

func TestParseMA0Answer890_ReadsThePrintedGrid(t *testing.T) {
	l := Layout890()
	rec, err := l.ParseMA0Answer([]byte(frame890("GB3IV")))
	if err != nil {
		t.Fatalf("ParseMA0Answer: %v", err)
	}
	want := Record{
		Slot: slotOf(t, l, 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3IV",
	}
	if rec != want {
		t.Errorf("ParseMA0Answer =\n %+v\nwant\n %+v", rec, want)
	}
}

// THE 890S FRAME LENGTH IS A RANGE AND THE ASSERTION IS CODEC-LOCAL.
// core/kw/ma CANNOT import core/driver/internal/drivertest: Go's internal
// rule confines that package to importers beneath core/driver, and this
// package is outside that tree (drivertest/kw_record_length.go:37-41 records
// the same boundary for core/kw, which keeps its own in-package assertion for
// the same reason). So the length contract is pinned here, and nothing waits
// on a helper this package can never call.
func TestParseMA0Answer890_TheFrameLengthIsThePrintedRange40To50(t *testing.T) {
	l := Layout890()
	for _, name := range []string{"", "ABC", "0123456789"} {
		f := frame890(name)
		if want := 40 + len(name); len(f) != want {
			t.Fatalf("the fixture for a %d-character name is %d bytes, want %d = 40 + len(name)", len(name), len(f), want)
		}
		if _, err := l.ParseMA0Answer([]byte(f)); err != nil {
			t.Errorf("a %d-byte answer was refused: %v", len(f), err)
		}
	}
	// An eleven-character name overruns the printed "Up to 10 characters"
	// (890:3208-3209) and the 50-byte maximum with it.
	if _, err := l.ParseMA0Answer([]byte(frame890("0123456789A"))); err == nil {
		t.Error("a 51-byte answer parsed; the printed maximum is 50")
	} else {
		assertParseRefusal(t, err, "40 to 50")
	}
	if _, err := l.ParseMA0Answer([]byte(frame890("")[:39])); err == nil {
		t.Error("a 39-byte frame parsed; the printed minimum is 40 (A17)")
	} else {
		assertParseRefusal(t, err, "40 to 50")
	}
}

// THE NORMATIVE ORDERING. The blank-window predicate runs BEFORE any
// per-field domain parse. A blank channel answers with P3 blank, and neither
// ' ' nor '0' is a mode either legend names, so a codec that parsed fields
// first would raise a ParseError on every unused slot — and clone.ReadAll
// returns on the FIRST channel error (core/clone/read.go:65-67), so ONE
// unused slot would make the radio unreadable end to end.
//
// THE MUTATION IS THE RED PROOF: move the predicate below the mode check in
// parseMA0Answer890 and this test fails.
func TestParseMA0Answer890_TheBlankWindowIsTestedBeforeAnyFieldDomain(t *testing.T) {
	l := Layout890()
	blank := "MA0007" + strings.Repeat(" ", 33) + ";"
	if len(blank) != 40 {
		t.Fatalf("the blank fixture is %d bytes, want 40", len(blank))
	}
	rec, err := l.ParseMA0Answer([]byte(blank))
	if err != nil {
		t.Fatalf("a blank channel must NOT be an error: %v", err)
	}
	if !rec.Empty {
		t.Error("a blank channel parsed as populated")
	}
	if rec.Mode != 0 || rec.FreqHz != 0 || rec.Name != "" {
		t.Errorf("a blank channel carried content: %+v", rec)
	}

	// A6 IS AN ASSUMPTION, so the predicate accepts all spaces OR all ASCII
	// '0': a radio that blanked with zeros would otherwise read as a real
	// channel tuned to 0 Hz, and zero hertz is not a channel on either chart.
	zeroed := "MA0007" + strings.Repeat("0", 33) + ";"
	rec, err = l.ParseMA0Answer([]byte(zeroed))
	if err != nil {
		t.Fatalf("an all-zero blank channel must NOT be an error: %v", err)
	}
	if !rec.Empty {
		t.Error("an all-zero blank channel parsed as populated")
	}
}

// A21: the blank-channel note stops at P12 (890:3215-3216, E4), so a fresh
// radio may answer a blank channel with a residue in its floating name
// window. Such a frame is reported UNASSIGNED with the residue carried in the
// record — never as an error, for the clone.ReadAll reason above, and never
// discarded silently.
func TestParseMA0Answer890_ABlankChannelWithANameResidueIsUnassignedNotAnError(t *testing.T) {
	l := Layout890()
	rec, err := l.ParseMA0Answer([]byte("MA0007" + strings.Repeat(" ", 33) + "RESIDUE;"))
	if err != nil {
		t.Fatalf("a blank channel with a name residue must NOT be an error: %v", err)
	}
	if !rec.Empty {
		t.Error("a blank channel with a name residue parsed as populated")
	}
	if rec.Name != "" {
		t.Errorf("Name = %q, want empty: P13 is outside the predicate and is not channel content (A21)", rec.Name)
	}
	if rec.NameResidue != "RESIDUE" {
		t.Errorf("NameResidue = %q, want %q — the residue is carried, not discarded", rec.NameResidue, "RESIDUE")
	}
}

func TestParseMA0Answer890_ReportsOutOfDomainDataAndNeverRepairsIt(t *testing.T) {
	l := Layout890()
	for _, tt := range []struct {
		what  string
		frame string
		want  string
	}{
		{"a missing prefix", "MA1" + frame890("AB")[3:], "MA0"},
		{"a missing terminator", frame890("AB")[:41] + "X", "terminator"},
		{"a non-digit channel number", with890("AB", 4, " "), "P1"},
		{"a non-digit frequency", with890("AB", 7, "X"), "P2"},
		{"a mode outside the legend", with890("AB", 18, "G"), "P3"},
		{"the Unused mode 0", with890("AB", 18, "0"), "P3"},
		{"the Unused mode 8", with890("AB", 18, "8"), "P3"},
		{"an FM width that is neither 0 nor 1", with890("AB", 19, "2"), "P4"},
		{"a tone type outside 0-3", with890("AB", 20, "4"), "P5"},
		{"a tone index above 50", with890("AB", 21, "51"), "P6"},
		{"a CTCSS index above 49", with890("AB", 23, "50"), "P7"},
		{"a split flag that is neither 0 nor 1", with890("AB", 38, "2"), "P11"},
		{"a lockout byte that is neither 0 nor 1", with890("AB", 39, "2"), "P12"},
		{"a name byte outside A2's charset", frame890("A\x01"), "P13"},
	} {
		t.Run(tt.what, func(t *testing.T) {
			_, err := l.ParseMA0Answer([]byte(tt.frame))
			assertParseRefusal(t, err, tt.want)
		})
	}
}

// A16 IS DOCUMENTED, NOT ASSUMED: "When reading a single memory channel, all
// parameters for Split Transmission become 0" (890:3217-3218). P9 is a mode
// byte and '0' is what BOTH legends print "Unused", so the secondary side is
// domain-checked only when it carries content — otherwise every unsplit
// channel on the radio would be unreadable.
func TestParseMA0Answer890_TheZeroedSplitSideIsThePrintedFormAndNotAModeRefusal(t *testing.T) {
	l := Layout890()
	rec, err := l.ParseMA0Answer([]byte(frame890("AB")))
	if err != nil {
		t.Fatalf("an unsplit channel with a zeroed split side was refused: %v", err)
	}
	if rec.TXMode != 0 || rec.TXFreqHz != 0 || rec.TXFMNarrow {
		t.Errorf("the zeroed split side produced %+v, want the secondary fields absent", rec)
	}
	// With content on the split side, P9 IS checked against the legend.
	split := with890("AB", 25, "00014260000")
	if _, err := l.ParseMA0Answer([]byte(split)); err == nil {
		t.Error("a populated split side with P9 = '0' parsed; '0' is Unused (890:3977)")
	} else {
		assertParseRefusal(t, err, "P9")
	}
}

func TestBuildMA0Set890_EmitsTheGridAndPadsNothing(t *testing.T) {
	l := Layout890()
	rec := Record{Slot: slotOf(t, l, 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3IV"}
	got := string(mustBuild(t, l, rec))
	if want := frame890("GB3IV"); got != want {
		t.Errorf("BuildMA0Set = %q, want %q", got, want)
	}
	// The terminator FLOATS: nothing is padded to a ten-byte window.
	short := rec
	short.Name = "A"
	if got, want := len(mustBuild(t, l, short)), 41; got != want {
		t.Errorf("a one-character name built %d bytes, want %d — this grid pads nothing", got, want)
	}

	// LOW-4 / A1: a trailing space cannot survive the round trip — the
	// terminator floats straight after the name, so "AB " builds the wire
	// form "AB ;" and parses back as "AB".
	trailing := rec
	trailing.Name = "AB "
	if got, want := string(mustBuild(t, l, trailing)), frame890("AB "); got != want {
		t.Errorf("BuildMA0Set with a trailing-space name = %q, want %q", got, want)
	}
	back, err := l.ParseMA0Answer(mustBuild(t, l, trailing))
	if err != nil || back.Name != "AB" {
		t.Errorf("round trip of a trailing-space name = %+v, %v, want Name %q", back, err, "AB")
	}
}

func TestBuildMA0Set890_Refusals(t *testing.T) {
	l := Layout890()
	ok := Record{Slot: slotOf(t, l, 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3IV"}
	// The positive control: the same record, unspoiled, reaches the wire.
	if _, err := l.BuildMA0Set(ok); err != nil {
		t.Fatalf("the positive control was refused: %v", err)
	}
	// LOW-2: Class '0' is Single (990:2897-2903) — precisely what an 890S
	// record already is — and the grid loses nothing this row didn't already
	// omit, unlike '1' Dual and '2' Section defined.
	if _, err := l.BuildMA0Set(spoil(ok, func(r *Record) { r.Class = '0' })); err != nil {
		t.Errorf("Class = '0' was refused: %v", err)
	}
	for _, tt := range []struct {
		what string
		rec  Record
		want string
	}{
		{"mode 0", withMode(ok, '0'), "Unused"},
		{"mode 8", withMode(ok, '8'), "Unused"},
		{"a mode outside this row's legend", withMode(ok, 'G'), "legend"},
		{"a tone index above TN's 50", spoil(ok, func(r *Record) { r.ToneIndex = 51 }), "TN"},
		{"a CTCSS index above CN's 49", spoil(ok, func(r *Record) { r.CTCSSIndex = 50 }), "CN"},
		{"a negative tone index", spoil(ok, func(r *Record) { r.ToneIndex = -1 }), "TN"},
		{"a name over ten characters", spoil(ok, func(r *Record) { r.Name = "ELEVENCHARS" }), "10 characters"},
		{"a name outside A2's charset", spoil(ok, func(r *Record) { r.Name = "A\x01" }), "A2"},
		{"a name containing ';'", spoil(ok, func(r *Record) { r.Name = "A;B" }), "';'"},
		{"a tone type outside 0-3", spoil(ok, func(r *Record) { r.ToneType = '4' }), "P5"},
		{"a frequency wider than eleven digits", spoil(ok, func(r *Record) { r.FreqHz = 100_000_000_000 }), "11 digits"},
		{"an empty record", spoil(ok, func(r *Record) { r.Empty = true }), "no erase"},
		{"the zero slot", spoil(ok, func(r *Record) { r.Slot = Slot{} }), "no slot"},
		{"a 990S dual-reception flag", spoil(ok, func(r *Record) { r.DualRecv = true }), "dual reception"},
		{"a 990S channel class", spoil(ok, func(r *Record) { r.Class = '1' }), "channel type"},
	} {
		t.Run(tt.what, func(t *testing.T) {
			_, err := l.BuildMA0Set(tt.rec)
			assertParseRefusal(t, err, tt.want)
		})
	}
	if _, err := (Layout{}).BuildMA0Set(ok); err == nil {
		t.Error("the zero layout built an MA0 Set")
	}
}

// spoil returns rec with f applied to a copy.
func spoil(rec Record, f func(*Record)) Record {
	f(&rec)
	return rec
}

// THE P4/P10 AGREEMENT RULE IS A CODEC INVARIANT AND IT IS UNREACHABLE FROM
// THE DRIVER. "When setting the split memory channel, set the same setting on
// the transmission side and the reception side for FM normal / narrow
// information (P4, P10)" (890:3219-3221). BuildMA0Set refuses a Record whose
// two sides disagree, and that refusal is worth keeping at the codec
// boundary — but codeplug.ChannelData carries ONE FM width, so the driver's
// builder emits P10 = P4 by construction and can never present such a record.
//
// ITS FIXTURE IS THEREFORE A HAND-BUILT ma.Record, NEVER A ChannelData: a
// rung whose red proof cannot be written honestly is not a rung.
func TestBuildMA0Set890_P4EqualsP10IsACodecInvariantTheDriverCannotReach(t *testing.T) {
	l := Layout890()
	split := Record{
		Slot: slotOf(t, l, 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "AB",
		TXFreqHz: 14_260_000, TXMode: '2', TXFMNarrow: true, FMNarrow: true, Split: true,
	}
	if _, err := l.BuildMA0Set(split); err != nil {
		t.Fatalf("the positive control (P4 = P10) was refused: %v", err)
	}
	disagree := split
	disagree.TXFMNarrow = false
	if _, err := l.BuildMA0Set(disagree); err == nil {
		t.Error("a record whose P4 and P10 disagree was built")
	} else {
		assertParseRefusal(t, err, "P4, P10")
	}
}

// THE EMISSION INVARIANT, asserted directly on the bytes: for every candidate
// the driver can build — one FM width, so TXFMNarrow tracks FMNarrow — the
// emitted P10 equals the emitted P4 whenever the record carries a split side.
// An unsplit candidate emits the printed ZEROED split side instead
// (890:3217-3218), which is what makes an ordinary channel round-trip with no
// refusal at all.
func TestBuildMA0Set890_EmittedP10EqualsEmittedP4(t *testing.T) {
	l := Layout890()
	for _, narrow := range []bool{false, true} {
		rec := Record{
			Slot: slotOf(t, l, 7), FreqHz: 145_000_000, Mode: '4', ToneType: '0', Name: "AB",
			FMNarrow: narrow, TXFreqHz: 145_600_000, TXMode: '4', TXFMNarrow: narrow, Split: true,
		}
		f := mustBuild(t, l, rec)
		if f[18] != f[36] {
			t.Errorf("narrow=%v: P4 = %q, P10 = %q — the emitted bytes must agree (890:3219-3221)", narrow, f[18], f[36])
		}
	}
	unsplit := Record{
		Slot: slotOf(t, l, 7), FreqHz: 145_000_000, Mode: '4', ToneType: '0', Name: "AB", FMNarrow: true,
	}
	f := mustBuild(t, l, unsplit)
	if got := string(f[24:38]); got != "000000000000"+"00" {
		t.Errorf("an unsplit candidate emitted P8-P11 = %q, want the printed zeroed split side (890:3217-3218)", got)
	}

	// MED-1: a record marked Split with a zeroed secondary side contradicts
	// itself — P11 says split (890:3201-3203) while P8-P10 print the single
	// memory channel's own zeroed form (890:3217-3218) — and must be
	// refused, not built.
	splitZeroed := unsplit
	splitZeroed.Split = true
	if _, err := l.BuildMA0Set(splitZeroed); err == nil {
		t.Error("a Split record with a zeroed secondary side was built")
	} else {
		assertParseRefusal(t, err, "P11")
	}
}
