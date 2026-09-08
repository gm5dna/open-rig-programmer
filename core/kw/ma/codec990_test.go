// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"strings"
	"testing"
)

// frame990 is a well-formed TS-990S MA0 answer for channel 007 with name in
// its FIXED ten-byte window at positions 47-56 (990:2919-2938). name must
// already be ten bytes.
func frame990(name string) string {
	f := "MA0" + // 1-3
		"007" + // P1, 4-6
		"0" + // P2, 7
		"00014250000" + // P3, 8-18
		"2" + // P4, 19
		"0" + // P5, 20
		"0" + // P6, 21
		"00" + // P7, 22-23
		"00" + // P8, 24-25
		"00000000000" + // P9, 26-36
		"0" + // P10, 37
		"0" + // P11, 38
		"0" + // P12, 39
		"00" + // P13, 40-41
		"00" + // P14, 42-43
		"0" + // P15, 44
		"0" + // P16, 45
		"1" + // P17, 46 — 1 is Scan Lockout OFF (E8)
		name + // P18, 47-56
		";" // 57
	return f
}

// with990 returns frame990's shape with the byte at 1-based position pos
// replaced by b.
func with990(pos int, b string) string {
	f := frame990("GB3       ")
	return f[:pos-1] + b + f[pos-1+len(b):]
}

func TestParseMA0Answer990_ReadsThePrintedGrid(t *testing.T) {
	l := Layout990()
	rec, err := l.ParseMA0Answer([]byte(frame990("GB3       ")))
	if err != nil {
		t.Fatalf("ParseMA0Answer: %v", err)
	}
	want := Record{
		Slot: slotOf(t, l, 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3", Class: '0',
	}
	if rec != want {
		t.Errorf("ParseMA0Answer =\n %+v\nwant\n %+v", rec, want)
	}
}

// THE 990S FRAME LENGTH IS EXACTLY 57 AND THE ASSERTION IS CODEC-LOCAL, for
// codec890_test.go's reason: this package is outside core/driver, so Go's
// internal rule puts core/driver/internal/drivertest permanently out of
// reach and nothing here waits on a helper it can never call.
func TestParseMA0Answer990_TheFrameLengthIsExactly57(t *testing.T) {
	l := Layout990()
	f := frame990("GB3       ")
	if len(f) != 57 {
		t.Fatalf("the fixture is %d bytes, want 57", len(f))
	}
	if _, err := l.ParseMA0Answer([]byte(f)); err != nil {
		t.Fatalf("the 57-byte answer was refused: %v", err)
	}
	for _, short := range []string{f[:56], f + "0"} {
		if _, err := l.ParseMA0Answer([]byte(short)); err == nil {
			t.Errorf("a %d-byte frame parsed; this grid is fixed at 57", len(short))
		} else {
			assertParseRefusal(t, err, "exactly 57")
		}
	}
}

// THE NORMATIVE ORDERING, and on this row the consequence is printed rather
// than derived: P17 is valid only as '1' or '2' (990:2952-2954) while a blank
// channel returns spaces across P2-P18 (990:2962-2963). A codec that parsed
// fields first would raise a ParseError on every unused slot, and
// clone.ReadAll returns on the FIRST channel error (core/clone/read.go:65-67).
//
// THE MUTATION IS THE RED PROOF: move the predicate below the first domain
// check in parseMA0Answer990 and this test fails.
func TestParseMA0Answer990_TheBlankWindowIsTestedBeforeAnyFieldDomain(t *testing.T) {
	l := Layout990()
	blank := "MA0007" + strings.Repeat(" ", 50) + ";"
	if len(blank) != 57 {
		t.Fatalf("the blank fixture is %d bytes, want 57", len(blank))
	}
	rec, err := l.ParseMA0Answer([]byte(blank))
	if err != nil {
		t.Fatalf("a blank channel must NOT be an error: %v", err)
	}
	if !rec.Empty {
		t.Error("a blank channel parsed as populated")
	}
	if rec.Name != "" || rec.Mode != 0 {
		t.Errorf("a blank channel carried content: %+v", rec)
	}
	zeroed := "MA0007" + strings.Repeat("0", 50) + ";"
	rec, err = l.ParseMA0Answer([]byte(zeroed))
	if err != nil {
		t.Fatalf("an all-zero blank channel must NOT be an error: %v", err)
	}
	if !rec.Empty {
		t.Error("an all-zero blank channel parsed as populated")
	}
}

// E8: this radio's scan lockout is "1: Scan Lockout OFF / 2: Scan Lockout ON"
// (990:2952-2954). A driver that "helpfully" accepted '0' would be importing
// MA3's 0/1 convention into MA0's field.
func TestParseMA0Answer990_RefusesALockoutByteThatIsNeither1Nor2(t *testing.T) {
	l := Layout990()
	for _, b := range []string{"0", "3", " "} {
		if _, err := l.ParseMA0Answer([]byte(with990(46, b))); err == nil {
			t.Errorf("P17 = %q parsed; E8 prints 1 and 2 only", b)
		} else {
			assertParseRefusal(t, err, "P17")
		}
	}
	on := with990(46, "2")
	rec, err := l.ParseMA0Answer([]byte(on))
	if err != nil {
		t.Fatalf("P17 = '2' was refused: %v", err)
	}
	if !rec.Lockout {
		t.Error("P17 = '2' did not read as Scan Lockout ON")
	}
}

func TestParseMA0Answer990_ReportsOutOfDomainDataAndNeverRepairsIt(t *testing.T) {
	l := Layout990()
	for _, tt := range []struct {
		what  string
		frame string
		want  string
	}{
		{"a missing prefix", "MA1" + frame990("GB3       ")[3:], "MA0"},
		{"a missing terminator", frame990("GB3       ")[:56] + "X", "terminator"},
		{"a channel type outside 0-2", with990(7, "3"), "P2"},
		{"a non-digit frequency", with990(8, "X"), "P3"},
		{"a mode outside the legend", with990(19, "O"), "P4"},
		{"the Unused mode 8", with990(19, "8"), "P4"},
		{"an FM width that is neither 0 nor 1", with990(20, "2"), "P5"},
		{"a tone function outside 0-3", with990(21, "4"), "P6"},
		{"a tone index above 50", with990(22, "51"), "P7"},
		{"a CTCSS index above 49", with990(24, "50"), "P8"},
		{"a split flag that is neither 0 nor 1", with990(44, "2"), "P15"},
		{"a dual-reception flag that is neither 0 nor 1", with990(45, "2"), "P16"},
		{"a name byte outside A2's charset", frame990("GB3\x01      "), "P18"},
	} {
		t.Run(tt.what, func(t *testing.T) {
			_, err := l.ParseMA0Answer([]byte(tt.frame))
			assertParseRefusal(t, err, tt.want)
		})
	}
}

// A1: the name lives in a FIXED ten-byte window, padded with ASCII space on
// write and right-trimmed on read. A14: P2 is the milestone's ONE defaulted
// byte, emitted '0' because the book says the parameter "is ignored. Enter a
// dummy value" (990:2901-2903) and names no value.
func TestBuildMA0Set990_PadsTheNameWindowAndEmitsP2AsZero(t *testing.T) {
	l := Layout990()
	rec := Record{Slot: slotOf(t, l, 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3"}
	got := mustBuild(t, l, rec)
	if want := frame990("GB3       "); string(got) != want {
		t.Errorf("BuildMA0Set = %q, want %q", got, want)
	}
	if string(got[46:56]) != "GB3       " {
		t.Errorf("P18 = %q, want the name padded to ten bytes with ASCII space (A1)", got[46:56])
	}
	if got[6] != '0' {
		t.Errorf("P2 = %q, want '0' (A14)", got[6])
	}
	full := rec
	full.Name = "0123456789"
	if got := mustBuild(t, l, full); string(got[46:56]) != "0123456789" {
		t.Errorf("a ten-character name emitted %q, want it whole", got[46:56])
	}
	// Right-trimmed on read, which is the other half of A1.
	back, err := l.ParseMA0Answer([]byte(frame990("GB3       ")))
	if err != nil {
		t.Fatalf("ParseMA0Answer: %v", err)
	}
	if back.Name != "GB3" {
		t.Errorf("Name = %q, want %q — trailing pad is not part of the name (A1)", back.Name, "GB3")
	}
}

func TestBuildMA0Set990_Refusals(t *testing.T) {
	l := Layout990()
	ok := Record{Slot: slotOf(t, l, 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3"}
	if _, err := l.BuildMA0Set(ok); err != nil {
		t.Fatalf("the positive control was refused: %v", err)
	}
	for _, tt := range []struct {
		what string
		rec  Record
		want string
	}{
		{"mode 0", withMode(ok, '0'), "Unused"},
		{"mode 8", withMode(ok, '8'), "Unused"},
		{"a mode outside this row's legend", withMode(ok, 'O'), "legend"},
		{"a tone index above TN's 50", spoil(ok, func(r *Record) { r.ToneIndex = 51 }), "TN"},
		{"a CTCSS index above CN's 49", spoil(ok, func(r *Record) { r.CTCSSIndex = 50 }), "CN"},
		{"a second tone index above TN's 50", spoil(ok, func(r *Record) { r.TXMode, r.TXToneType, r.TXToneIndex = '2', '1', 51 }), "TN"},
		{"a name over ten characters", spoil(ok, func(r *Record) { r.Name = "ELEVENCHARS" }), "10 characters"},
		{"a name outside A2's charset", spoil(ok, func(r *Record) { r.Name = "A\x01" }), "A2"},
		{"a name containing ';'", spoil(ok, func(r *Record) { r.Name = "A;B" }), "';'"},
		{"a second tone type outside 0-3", spoil(ok, func(r *Record) { r.TXMode, r.TXToneType = '2', '4' }), "P12"},
		{"an empty record", spoil(ok, func(r *Record) { r.Empty = true }), "no erase"},
		{"the zero slot", spoil(ok, func(r *Record) { r.Slot = Slot{} }), "no slot"},
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

// A16 on this row: "When reading a single memory channel, all parameters for
// frequency 2 become 0" (990:2964-2965). P10 is a mode byte and '0' is
// "Unused" (990:3707), so the frequency-2 side is domain-checked only when it
// carries content — and the codec re-emits the printed zeroed form for a
// record that carries none, which is what makes an ordinary channel
// round-trip with no refusal.
func TestBuildMA0Set990_TheAbsentSecondSideIsEmittedAsThePrintedZeroedForm(t *testing.T) {
	l := Layout990()
	rec := Record{Slot: slotOf(t, l, 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3"}
	f := mustBuild(t, l, rec)
	if got := string(f[25:43]); got != strings.Repeat("0", 18) {
		t.Errorf("P9-P14 = %q, want eighteen zeros (990:2964-2965)", got)
	}
	// A record claiming no second side but carrying one is refused rather
	// than half-emitted.
	bad := rec
	bad.TXFreqHz = 14_260_000
	if _, err := l.BuildMA0Set(bad); err == nil {
		t.Error("a record with a frequency 2 but no mode 2 was built")
	} else {
		assertParseRefusal(t, err, "second side")
	}
}
