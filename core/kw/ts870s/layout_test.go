// SPDX-License-Identifier: GPL-3.0-or-later

package ts870s_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts870s"
)

// TestLayout_Values pins the row's own settled facts, so a later edit to
// layout.go that drifted from the matrix fails loudly here rather than
// only in a round-trip test that would not notice a wrong Model or menu
// ceiling.
func TestLayout_Values(t *testing.T) {
	l := ts870s.Layout
	if !l.Configured() {
		t.Fatal("ts870s.Layout is the zero Layout870")
	}
	if l.Model() != "TS-870S" {
		t.Errorf("Model() = %q, want %q", l.Model(), "TS-870S")
	}
	if l.MaxEXAddress() != 68 {
		t.Errorf("MaxEXAddress() = %d, want 68 (doc.go's own citation, ts870s:8468-8508)", l.MaxEXAddress())
	}
	names := l.ModeNames()
	if len(names) != 8 {
		t.Fatalf("ModeNames() has %d entries, want 8 (matrix §1.5: ten Format 2 nibbles minus the two holes)", len(names))
	}
	for _, hole := range []kw.Mode{kw.ModeNone, kw.ModeTune} {
		if _, ok := names[hole]; ok {
			t.Errorf("ModeNames() names %v, which Format 2 marks as a hole, not a mode", hole)
		}
	}
}

// conformanceRecord is a populated channel this row's own grid can carry:
// 14.250 MHz, LSB, unlocked, tone on at index 1.
func conformanceRecord() kw.Record870 {
	return kw.Record870{
		Channel:   1,
		FreqHz:    14_250_000,
		Mode:      kw.ModeLSB,
		Lockout:   '0',
		ToneMode:  kw.ToneModeTone,
		ToneIndex: 1,
	}
}

// TestBuildMWSet_RoundTrip holds the row to the same build -> parse
// property kwtest.Run holds every kw.Layout to; kwtest.Run itself cannot
// be called here because it takes a kw.Layout and ts870s.Layout is a
// kw.Layout870, a distinct type (record870.go's own doc comment) — this
// is the hand-rolled equivalent for the second record type.
func TestBuildMWSet_RoundTrip(t *testing.T) {
	l := ts870s.Layout
	rec := conformanceRecord()

	cmd, err := l.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	frame := cmd.Bytes()
	if len(frame) != 22 {
		t.Fatalf("BuildMWSet produced %d bytes, want 22 (matrix: total record width)", len(frame))
	}
	if frame[0] != 'M' || frame[1] != 'W' {
		t.Fatalf("BuildMWSet frame = %q, want an MW prefix", frame)
	}
	if frame[len(frame)-1] != ';' {
		t.Fatalf("BuildMWSet frame = %q, missing terminator", frame)
	}

	answer := append([]byte{}, frame...)
	answer[0], answer[1] = 'M', 'R'
	got, err := l.ParseMRAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMRAnswer refused this layout's own MW frame %q: %v", answer, err)
	}
	if got != rec {
		t.Errorf("round trip: sent %+v, got back %+v", rec, got)
	}
}

// TestBuildMWSet_Refusals holds the outbound builder to the domain
// record870.go itself enforces, per row.
func TestBuildMWSet_Refusals(t *testing.T) {
	l := ts870s.Layout
	base := conformanceRecord()

	cases := []struct {
		name string
		rec  kw.Record870
	}{
		{"channel below range", func() kw.Record870 { r := base; r.Channel = -1; return r }()},
		{"channel above range", func() kw.Record870 { r := base; r.Channel = 100; return r }()},
		{"zero frequency", func() kw.Record870 { r := base; r.FreqHz = 0; return r }()},
		{"unnamed mode (ModeNone)", func() kw.Record870 { r := base; r.Mode = kw.ModeNone; return r }()},
		{"unnamed mode (ModeTune)", func() kw.Record870 { r := base; r.Mode = kw.ModeTune; return r }()},
		{"invalid lockout byte", func() kw.Record870 { r := base; r.Lockout = '2'; return r }()},
		{"invalid tone mode (CTCSS, not on this row)", func() kw.Record870 { r := base; r.ToneMode = kw.ToneModeCTCSS; return r }()},
		{"tone index below this row's own chart", func() kw.Record870 { r := base; r.ToneIndex = 0; return r }()},
		{"tone index above this row's own 39-entry chart (40, inside the family's old 00-42 bound)", func() kw.Record870 { r := base; r.ToneIndex = 40; return r }()},
		{"Empty record: this codec builds no erase/empty MW frame", func() kw.Record870 { r := base; r.Empty = true; return r }()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.BuildMWSet(tc.rec); err == nil {
				t.Errorf("BuildMWSet(%+v) built a frame, want a refusal", tc.rec)
			}
		})
	}
}

// TestParseMRAnswer_Refusals holds the inbound parser to the same domain
// from the wire side, plus the frame-shape checks record870.go states are
// its own (prefix, width, terminator, P1).
func TestParseMRAnswer_Refusals(t *testing.T) {
	l := ts870s.Layout
	good, err := l.BuildMWSet(conformanceRecord())
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	answer := good.Bytes()
	answer[0], answer[1] = 'M', 'R'

	mutate := func(pos int, b byte) []byte {
		out := append([]byte{}, answer...)
		out[pos] = b
		return out
	}

	cases := []struct {
		name  string
		frame []byte
	}{
		{"wrong length", answer[:21]},
		{"wrong prefix", mutate(0, 'X')},
		{"missing terminator", mutate(21, '0')},
		{"P1 not '0' (channel 99's Start/End half, unreachable here)", mutate(2, '1')},
		{"unnamed mode nibble '0' (No mode)", mutate(16, '0')},
		{"unnamed mode nibble '8' (No Mode)", mutate(16, '8')},
		{"undocumented mode nibble", mutate(16, 'Z')},
		{"invalid lockout byte", mutate(17, '2')},
		{"invalid tone-mode byte (CTCSS, not on this row)", mutate(18, '2')},
	}
	// P8 (positions 20-21, 0-indexed 19-20) "40": inside the family's old
	// 00-42 bound, outside this row's own 01-39 chart (matrix §1.9) — the
	// P8-bound fix's own pin.
	tone40 := append([]byte{}, answer...)
	tone40[19], tone40[20] = '4', '0'
	cases = append(cases, struct {
		name  string
		frame []byte
	}{"tone index 40, past this row's own 39-entry chart", tone40})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.ParseMRAnswer(tc.frame); err == nil {
				t.Errorf("ParseMRAnswer(%q) succeeded, want a refusal", tc.frame)
			}
		})
	}
}

// TestParseMRAnswer_VacantChannelRoundTrips pins the fix the Lift K
// follow-up (13/09/2026) landed in record870.go: "For a vacant channel,
// the Answer command sends '0' for all parameters except the memory
// channel number." (ts870s:9101-9104, matrix §1.15/§2.1). Channel 5's own
// digits survive; P4-P8 all read zero and decode as Empty rather than
// being refused at the mode-legend check.
func TestParseMRAnswer_VacantChannelRoundTrips(t *testing.T) {
	l := ts870s.Layout
	good, err := l.BuildMWSet(conformanceRecord())
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	vacant := good.Bytes()
	vacant[0], vacant[1] = 'M', 'R'
	copy(vacant[3:5], "05")
	for i := 5; i <= 20; i++ {
		vacant[i] = '0'
	}
	rec, err := l.ParseMRAnswer(vacant)
	if err != nil {
		t.Fatalf("ParseMRAnswer(%q): %v", vacant, err)
	}
	if !rec.Empty || rec.Channel != 5 {
		t.Errorf("ParseMRAnswer(%q) = %+v, want Empty=true, Channel=5", vacant, rec)
	}
}

// TestBuildIDRead pins the shared identity grammar (matrix §1.2).
func TestBuildIDRead(t *testing.T) {
	l := ts870s.Layout
	cmd, err := l.BuildIDRead()
	if err != nil {
		t.Fatalf("BuildIDRead: %v", err)
	}
	if got := string(cmd.Bytes()); got != "ID;" {
		t.Errorf("BuildIDRead = %q, want %q", got, "ID;")
	}
	got, err := l.ParseIDAnswer([]byte("ID015;"))
	if err != nil {
		t.Fatalf("ParseIDAnswer: %v", err)
	}
	if got != "015" {
		t.Errorf("ParseIDAnswer = %q, want %q (matrix §1.2)", got, "015")
	}
	if _, err := l.ParseIDAnswer([]byte("ID08;")); err == nil {
		t.Error("ParseIDAnswer accepted a five-byte frame")
	}
}

// TestBuildMRRead_RoundTripsThroughAllowedCommand pins the read grammar
// AllowedCommand now admits (Lift K follow-up, 13/09/2026): the frame
// BuildMRRead produces passes the outbound gate NewFramingFor870 uses, and
// the matcher it hands a session correlates it to the right channel and no
// other.
func TestBuildMRRead_RoundTripsThroughAllowedCommand(t *testing.T) {
	l := ts870s.Layout
	cmd, err := l.BuildMRRead(7)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}
	frame := cmd.Bytes()
	if got := string(frame); got != "MR007;" {
		t.Errorf("BuildMRRead(7) = %q, want %q", got, "MR007;")
	}
	if !l.AllowedCommand(frame) {
		t.Errorf("AllowedCommand refused %q, its own builder's output", frame)
	}
	if _, err := l.BuildMRRead(100); err == nil {
		t.Error("BuildMRRead(100) succeeded, channel 100 is outside the 0-99 bank")
	}

	// MRAnswerMatcher correlates the 22-byte ANSWER a radio sends, not
	// the 6-byte READ request above — the two are different frames on
	// the wire, as they are for the family's own MRAnswerMatcher.
	rec := conformanceRecord()
	rec.Channel = 7
	mw, err := l.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	answer := mw.Bytes()
	answer[0], answer[1] = 'M', 'R'
	if !l.MRAnswerMatcher(7)(answer) {
		t.Error("MRAnswerMatcher(7) did not match channel 7's own answer")
	}
	if l.MRAnswerMatcher(8)(answer) {
		t.Error("MRAnswerMatcher(8) matched channel 7's answer")
	}
}

// TestAllowedCommand_AdmitsTheFourGrammarsAndNothingElse is the gate's own
// negative space: everything AllowedCommand's doc comment says it refuses.
func TestAllowedCommand_AdmitsTheFourGrammarsAndNothingElse(t *testing.T) {
	l := ts870s.Layout
	mw, err := l.BuildMWSet(conformanceRecord())
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	for _, frame := range [][]byte{[]byte("AI0;"), []byte("ID;"), mw.Bytes()} {
		if !l.AllowedCommand(frame) {
			t.Errorf("AllowedCommand refused %q, one of the four admitted grammars", frame)
		}
	}
	mrCmd, err := l.BuildMRRead(1)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}
	if !l.AllowedCommand(mrCmd.Bytes()) {
		t.Errorf("AllowedCommand refused %q, one of the four admitted grammars", mrCmd.Bytes())
	}

	answer := append([]byte{}, mw.Bytes()...)
	answer[0], answer[1] = 'M', 'R'
	for _, tc := range []struct {
		name  string
		frame []byte
	}{
		{"the MR answer (never outbound)", answer},
		{"an ID answer", []byte("ID015;")},
		{"an AI read (never built)", []byte("AI;")},
		{"a non-zero AI state (never built)", []byte("AI1;")},
		{"an EX read, unbuilt on this row", []byte("EX0000000;")},
		{"a mutated MW (mode byte replaced with an undocumented nibble)", func() []byte { f := append([]byte{}, mw.Bytes()...); f[16] = 'Z'; return f }()},
	} {
		if l.AllowedCommand(tc.frame) {
			t.Errorf("%s: AllowedCommand admitted %q", tc.name, tc.frame)
		}
	}
}
