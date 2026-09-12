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
		{"tone index below range", func() kw.Record870 { r := base; r.ToneIndex = -1; return r }()},
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
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.ParseMRAnswer(tc.frame); err == nil {
				t.Errorf("ParseMRAnswer(%q) succeeded, want a refusal", tc.frame)
			}
		})
	}
}

// TestParseMRAnswer_VacantChannelIsRefused_KnownGap pins a real, current
// limitation rather than papering over it: unlike the family's kw.Record,
// Record870 has no "P4-P8 all zero" empty-channel pre-check (record870.go
// carries none, and this package does not add one — it does not own that
// file). The manual's own vacant-channel sentence ("the Answer command
// sends '0' for all parameters except the memory channel number",
// ts870s:9101-9104, matrix §1.15/§2.1) therefore does not yet round-trip
// through this codec: a genuinely vacant channel's mode byte is '0', which
// is not in ModeNames, so ParseMRAnswer refuses it. This test exists so a
// future change to record870.go's empty-channel handling is a deliberate,
// visible edit here rather than a silent behaviour change.
func TestParseMRAnswer_VacantChannelIsRefused_KnownGap(t *testing.T) {
	l := ts870s.Layout
	good, err := l.BuildMWSet(conformanceRecord())
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	vacant := good.Bytes()
	vacant[0], vacant[1] = 'M', 'R'
	for i := 3; i <= 20; i++ {
		vacant[i] = '0'
	}
	if _, err := l.ParseMRAnswer(vacant); err == nil {
		t.Error("ParseMRAnswer accepted an all-zero vacant channel — record870.go has grown empty-channel handling; update doc.go's noted gap and this test together")
	}
}
