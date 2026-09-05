// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mwFrame590 is a valid 50-byte TS-590SG MW Set, built by the builder
// itself so that the gate is judged against a frame this codec really
// emits rather than one a test hand-assembled.
func mwFrame590(t *testing.T) []byte {
	t.Helper()
	l := layout590SG()
	cmd, err := l.BuildMWSet(populatedRecord(mustSlot(t, l, 7, ScanHalfNone)))
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	return cmd.Bytes()
}

// TestAllowedCommand_AcceptsExactlyTheEightGrammars walks every frame this
// codec builds, on every row, and requires its own row's gate to admit it.
//
// A BUILDER AND A GATE THAT DISAGREE ARE WORSE THAN EITHER BEING WRONG
// ALONE: the programme either cannot send a command it believes is valid, or
// the gate is not checking what the builder emits.
func TestAllowedCommand_AcceptsExactlyTheEightGrammars(t *testing.T) {
	for _, l := range []Layout{layout590SG(), layout590S(), layout480()} {
		admitted := 0
		admit := func(what string, build func() (Command, error)) {
			t.Helper()
			cmd, err := build()
			if err != nil {
				t.Errorf("%s: %s: %v", l.Model(), what, err)
				return
			}
			admitted++
			if !l.AllowedCommand(cmd.Bytes()) {
				t.Errorf("%s: its own gate REFUSED the %s frame %q that its own builder produced", l.Model(), what, cmd.Bytes())
			}
		}

		admit("ID read", l.BuildIDRead)
		admit("AI read", l.BuildAIRead)
		admit("AI set", l.BuildAISetOff)
		admit("MC read", l.BuildMCRead)
		admit("MC set", func() (Command, error) { return l.BuildMCSet(mustSlot(t, l, 7, ScanHalfNone)) })
		admit("MR read", func() (Command, error) { return l.BuildMRRead(mustSlot(t, l, 7, ScanHalfNone)) })
		admit("MW set", func() (Command, error) {
			return l.BuildMWSet(populatedRecord(mustSlot(t, l, 7, ScanHalfNone)))
		})
		admit("EX read", func() (Command, error) { return l.BuildEXRead(EXAddress{P1: 2}) })
		if l.Book() == Book590 {
			admit("FV read", l.BuildFVRead)
		} else {
			admit("TY read", l.BuildTYRead)
		}

		// A vacuity guard: a Configured() that had silently become false, or
		// a builder loop that stopped running, would leave every assertion
		// above unexecuted.
		if admitted != 9 {
			t.Errorf("%s: %d builder outputs reached the gate, want 9 (the eight grammars, of which AI and MC each contribute two frames and the row contributes one of FV/TY)", l.Model(), admitted)
		}
	}
}

// TestAllowedCommand_RefusesEveryAnswerFrameExceptTheTwoDisclosedCases. An
// answer is refused unless it is byte-identical to a Set this codec builds
// — and only "AI0;" and an MC naming ordinary memory ever are (both
// disclosed in AllowedCommand's doc comment and pinned by their own tests).
// Every OTHER answer frame this gate could be shown, however well formed,
// is refused — because admitting one would let a captured reply be written
// back — and this test is the negative case for all of them.
//
// MC IS THE CASE THAT MATTERS EVEN HERE: its Set and its Answer are the
// same six bytes exactly, so the gate cannot tell them apart by shape and
// judges the frame against the SEND domain instead. "MC110;" and "MC100;"
// below are that pin's refusing half: legitimate TS-590SG ANSWERS naming an
// extension or section-defined channel, refused outbound because A16
// narrows the Set to ordinary memory — the narrower domain that makes an MC
// answer naming ORDINARY memory (not tested as a refusal here; see
// validMCCommand's doc comment) the gate's second disclosed admission.
func TestAllowedCommand_RefusesEveryAnswerFrameExceptTheTwoDisclosedCases(t *testing.T) {
	sg, t480 := layout590SG(), layout480()

	for _, tc := range []struct {
		l     Layout
		frame string
		why   string
	}{
		{sg, "ID023;", "the ID answer (590:1119)"},
		{t480, "ID020;", "the ID answer (480:687)"},
		{sg, "FV1.00;", "the FV answer (590:1037)"},
		{t480, "TY001;", "the TY answer (480:1634)"},
		{sg, "AI0", "a truncated AI"},
		{sg, "MC110;", "an MC answer naming an extension channel — A16 narrows the SET to ordinary memory, and the two directions share one wire shape"},
		{sg, "MC100;", "an MC answer naming a section-defined channel"},
		{sg, "EX00200003;", "an EX answer, whose shape is the Set's (590:556-560)"},
		{t480, "EX032000005;", "an EX answer with a two-character P5 (480:413-416)"},
	} {
		if tc.l.AllowedCommand([]byte(tc.frame)) {
			t.Errorf("%s: the gate ADMITTED %q — %s", tc.l.Model(), tc.frame, tc.why)
		}
	}

	// The 50-byte MR ANSWER, which is the MW Set's own wire shape with a
	// different prefix. It must be refused even though this codec parses it
	// happily inbound.
	answer := []byte(answer590().frame(t))
	if sg.AllowedCommand(answer) {
		t.Errorf("the gate ADMITTED a 50-byte MR answer %q; MR has no Set on either radio (480:911, erratum E17)", answer)
	}
}

// TestAllowedCommand_RefusesAnEmbeddedSemicolon. Exactly one command must
// reach the wire per call: a second terminator anywhere splits one frame
// into two on the radio's own parser, and the 590 book says of the memory
// name that "';' cannot be used" (590:1577).
func TestAllowedCommand_RefusesAnEmbeddedSemicolon(t *testing.T) {
	sg := layout590SG()
	for _, frame := range []string{
		"ID;ID;",
		"MC;MC003;",
		"AI0;AI2;",
		"EX0020000;;",
		";ID;",
	} {
		if sg.AllowedCommand([]byte(frame)) {
			t.Errorf("the gate ADMITTED %q, which carries more than one terminator", frame)
		}
	}

	// And inside a field: an MW whose memory name hides one. The frame is
	// still fifty bytes, so only a field-by-field re-validation catches it.
	f := answer590()
	f.prefix, f.p16 = "MW", "A;C     "
	if sg.AllowedCommand([]byte(f.frame(t))) {
		t.Error("the gate ADMITTED a 50-byte MW whose P16 carries a ';' (590:1577)")
	}
}

// TestAllowedCommand_TheNegativePins is the plan's list, each entry a frame
// that must never leave this host.
func TestAllowedCommand_TheNegativePins(t *testing.T) {
	sg, s, t480 := layout590SG(), layout590S(), layout480()

	for _, tc := range []struct {
		l     Layout
		frame string
		why   string
	}{
		// No MT frame: there is no such command in either book.
		{sg, "MT001;", "MT is a Yaesu command and neither Kenwood book has one — the channel name is field P16 of MR/MW"},
		{sg, "MT;", "the same, in its read shape"},
		// No MA frame: the 890S/990S family is out of scope.
		{sg, "MA;", "MA belongs to the 890S/990S family, which this milestone does not register"},
		// No XI/XT frame: A20 records that the 480's own charts disagree
		// about them, and this milestone builds and parses neither.
		{t480, "XI;", "XI is unbuilt and unparsed — A20's paste-error reading is unlifted, and its lift needs a diagnostic outside this programme"},
		{t480, "XT;", "XT likewise"},
		// No discovery frame of any kind (decision 5): the slot space is
		// fully printed on both radios, so there is nothing to discover.
		{sg, "IF;", "IF is a transceiver-status read this milestone never issues"},
		{sg, "FA;", "no frequency read is built"},
		{sg, "SU0;", "the 480's scan-group selector, and no group frame is built on any row"},
		{sg, "PS1;", "the power frame of the 480's wake-up string, a non-goal (480:1149)"},
		// No AI state but 0.
		{sg, "AI1;", "AI1 is not even in the 590 pair's legend (590:159-162)"},
		{sg, "AI2;", "AI2 turns Auto Information ON, pushing a response per changed parameter (590:165-167)"},
		{sg, "AI4;", "AI4 turns it on with backup (590:162)"},
		{t480, "AI1;", "AI1 pushes an IF frame every 1.5 s while the IF parameters change (480:194-195)"},
		{t480, "AI3;", "AI3 turns both formats on (480:190)"},
		// MC Set outside ordinary memory (A16).
		{sg, "MC109;", "the last section-defined channel"},
		{sg, "MC119;", "the last extension channel"},
		// A frame legal on the OTHER book.
		{t480, "FV;", "FV appears nowhere in the 2003 TS-480 document"},
		{sg, "TY;", "TY appears nowhere in the TS-590S/SG document"},
		{s, "MC110;", "110 is outside the TS-590S's slot space (A12)"},
		// The Yaesu identity answer.
		{sg, "ID0800;", "the FT-710's own seven-byte, four-digit ID answer — a Kenwood answer is six bytes with three digits"},
	} {
		if tc.l.AllowedCommand([]byte(tc.frame)) {
			t.Errorf("%s: the gate ADMITTED %q — %s", tc.l.Model(), tc.frame, tc.why)
		}
	}
}

// TestAllowedCommand_RefusesEveryMWWidthButFifty is the erase form's own
// pin. 590:1579-1581 describes a short MW that ERASES the channel; its
// length is a reading rather than a printed number (A5, erratum E19); this
// milestone never builds it (decision 8), and the gate is what stops one
// reaching the wire from anywhere else.
func TestAllowedCommand_RefusesEveryMWWidthButFifty(t *testing.T) {
	sg := layout590SG()
	full := mwFrame590(t)
	if !sg.AllowedCommand(full) {
		t.Fatalf("the gate refused its own builder's 50-byte MW %q", full)
	}
	// The 42-byte erase shape, P16 wholly omitted, and every other width
	// down to the shortest thing with an "MW" prefix.
	short := append(append([]byte{}, full[:recNameOff]...), ';')
	if sg.AllowedCommand(short) {
		t.Errorf("the gate ADMITTED a %d-byte MW %q — the short form of 590:1579-1581 ERASES the channel", len(short), short)
	}
	for _, n := range []int{3, 7, 41, 49, 51} {
		frame := make([]byte, n)
		copy(frame, "MW")
		for i := 2; i < n-1; i++ {
			frame[i] = '0'
		}
		frame[n-1] = ';'
		if sg.AllowedCommand(frame) {
			t.Errorf("the gate ADMITTED a %d-byte MW %q", n, frame)
		}
	}
}

// TestAllowedCommand_RefusesAnMWTheBuilderWouldNotHaveEmitted is the
// re-validation the gate's whole claim rests on: a frame of the right width
// with the right prefix whose CONTENT no builder would produce.
func TestAllowedCommand_RefusesAnMWTheBuilderWouldNotHaveEmitted(t *testing.T) {
	sg := layout590SG()
	for _, tc := range []struct {
		spoil func(f *recordFields)
		why   string
	}{
		{func(f *recordFields) { f.p10 = "001" }, "P10 is hard-wired \"000\" in both books (590:1558-1559)"},
		{func(f *recordFields) { f.p13 = "000000001" }, "P13 is hard-wired \"000000000\" (590:1567-1568)"},
		{func(f *recordFields) { f.p5 = "0" }, "mode nibble '0' is \"None (setting failure)\" (590:1353, A18b)"},
		{func(f *recordFields) { f.p5 = "8" }, "mode nibble '8' likewise (590:1362, A18b)"},
		{func(f *recordFields) { f.p8 = "43" }, "tone index 43 is past TN's printed chart (590:2291, A21)"},
		{func(f *recordFields) { f.p9 = "42" }, "CTCSS index 42 is past CN's printed chart (590:411, A21)"},
		{func(f *recordFields) { f.p7 = "4" }, "P7 prints four values and stops at '3' (590:1549-1553)"},
		{func(f *recordFields) { f.p14 = "02" }, "P14 prints only \"00\" and \"01\" on the 590 pair (590:1569-1571)"},
		{func(f *recordFields) { f.p2 = " " }, "the space is A10's ASSUMED reading of a chart that only says \"refer to the MC command\"; this codec always emits '0' on MR and MW, and the gate admits what it builds"},
		{func(f *recordFields) { f.p4 = "0000000000 " }, "a space inside the frequency field"},
		{func(f *recordFields) { f.p16 = "TEST\x01   " }, "a control byte in the memory name (A2)"},
		{func(f *recordFields) {
			f.p4, f.p5, f.p6, f.p7, f.p8, f.p9, f.p11, f.p14, f.p15 = "00000000000", "0", "0", "0", "00", "00", "0", "00", "0"
			f.p16 = "        "
		}, "the EMPTY channel of 590:1492-1493 — a record this codec never writes, because the only documented clear is the short MW it never builds"},
	} {
		f := answer590()
		f.prefix = "MW"
		tc.spoil(&f)
		frame := []byte(f.frame(t))
		if len(frame) != RecordLen {
			t.Fatalf("the fixture is %d bytes, not %d; the case would prove nothing about width", len(frame), RecordLen)
		}
		if sg.AllowedCommand(frame) {
			t.Errorf("the gate ADMITTED a 50-byte MW %q — %s", frame, tc.why)
		}
	}

	// The positive control: the same fixture unspoiled must be admitted, or
	// every case above passes for the wrong reason.
	f := answer590()
	f.prefix = "MW"
	if !sg.AllowedCommand([]byte(f.frame(t))) {
		t.Error("the gate refused the unspoiled fixture, so every refusal above proves nothing")
	}
}

// TestAllowedCommand_RefusesAnMROrdinaryMemoryReadWithP1EqualsOne. The MR
// read's P1 is derived from the slot's CLASS and never chosen freely (M9,
// Slot.P1), so "MR1007;" is a frame no builder can produce and the gate
// refuses it. "Simplex" is A9's own word for the frame this refuses and is
// not a property any Layout slot carries — SlotMemory is the class, and it
// has no split/simplex axis at all — so the refusal is stated here in terms
// of what the gate actually checks: P1=1 on an ORDINARY MEMORY channel,
// which is unbuildable and, per 590:1444-1447 and 480:951, ambiguous with
// the documented read of a split channel's transmit frequency (see
// validMRCommand's doc comment for the full reasoning). The SCAN 'U' read
// beside it is the positive control: P1='1' is legal there, and it is a
// DOCUMENTED read (590:1449-1451).
func TestAllowedCommand_RefusesAnMROrdinaryMemoryReadWithP1EqualsOne(t *testing.T) {
	sg := layout590SG()
	if sg.AllowedCommand([]byte("MR1007;")) {
		t.Error("the gate ADMITTED \"MR1007;\": channel 007 is ordinary memory, whose class derives P1='0', and an MR with P1=1 on an ordinary-memory channel is unbuildable and ambiguous with a split channel's TX-frequency read")
	}
	if !sg.AllowedCommand([]byte("MR0007;")) {
		t.Error("the gate refused \"MR0007;\", the read this codec issues for every MEM slot (P13)")
	}
	if !sg.AllowedCommand([]byte("MR1100;")) {
		t.Error("the gate refused \"MR1100;\", the END frequency of section-defined channel 100 (590:1449-1451)")
	}
	if !sg.AllowedCommand([]byte("MR0100;")) {
		t.Error("the gate refused \"MR0100;\", that channel's START frequency")
	}
}

// TestAllowedCommand_TheOneDeliberateWideningIsMCsPrintedSPACE, and it is
// the line this gate draws between a spelling a MANUFACTURER prints and one
// this codec ASSUMES.
//
// MC's own chart prints both: "enter 0 or a space for a channel number less
// than 100" (590:1334-1335). So a space there is a documented Set spelling
// and the gate admits it even though the builder always emits '0' — refusing
// it would be this codec inventing a rule the book does not state.
//
// MR and MW get the opposite answer, and TestAllowedCommand_RefusesAnMW…
// pins it: those two charts say only "refer to the MC command"
// (590:1452-1453, 590:1539-1540), so that they share the convention at all
// is A10, ASSUMED. A gate does not widen itself on an assumption.
func TestAllowedCommand_TheOneDeliberateWideningIsMCsPrintedSPACE(t *testing.T) {
	sg, t480 := layout590SG(), layout480()
	if !sg.AllowedCommand([]byte("MC 03;")) {
		t.Error("the gate refused \"MC 03;\", the space spelling MC's own Set chart prints (590:1334-1335)")
	}
	if !sg.AllowedCommand([]byte("MC003;")) {
		t.Error("the gate refused \"MC003;\", the spelling this codec emits")
	}
	// The 480 prints no space convention at all: "0: Always 0 for the
	// TS-480 (Memory bank number)" (480:827).
	if t480.AllowedCommand([]byte("MC 03;")) {
		t.Error("the TS-480 gate ADMITTED a space in position 3, which its own book prints \"Always 0\" (480:827)")
	}
}

// TestAllowedCommand_GatesForTheLayoutItIsCalledOn. A gate that
// re-validated against a package-level datum would accept, on any radio,
// whatever one radio accepts — a safety failure rather than merely a
// correctness one.
func TestAllowedCommand_GatesForTheLayoutItIsCalledOn(t *testing.T) {
	sg, s, t480 := layout590SG(), layout590S(), layout480()

	// A frame legal on the SG and on no other row.
	if !sg.AllowedCommand([]byte("MR0110;")) {
		t.Error("the TS-590SG gate refused a read of its own extension channel 110")
	}
	if s.AllowedCommand([]byte("MR0110;")) {
		t.Error("the TS-590S gate ADMITTED a read of 110, which A12 leaves outside its slot space")
	}
	if t480.AllowedCommand([]byte("MR0110;")) {
		t.Error("the TS-480 gate ADMITTED a read of 110, and its channel number is two digits (480:955)")
	}

	// The 480's own MW carries a P14 no 590 row admits, and the reverse.
	f480 := answer480()
	f480.prefix = "MW"
	if !t480.AllowedCommand([]byte(f480.frame(t))) {
		t.Error("the TS-480 gate refused its own row's MW")
	}
	if sg.AllowedCommand([]byte(f480.frame(t))) {
		t.Error("the TS-590SG gate ADMITTED a TS-480 MW whose P14 is an ST step index (480:979); the 590 pair print only \"00\" and \"01\" there")
	}
}

// TestAllowedCommand_ZeroLayoutAdmitsNothing. A zero Layout is constructible
// by any caller and its AllowedCommand is a non-nil method value, so nothing
// about the method's existence says which radio it speaks for. The
// class-aware checks give the refusal for free wherever slot data is
// consulted — an empty slot space matches no slot — but "ID;", "AI;",
// "AI0;", "MC;" and "EX0020000;" consult no layout datum at all and would
// otherwise pass.
func TestAllowedCommand_ZeroLayoutAdmitsNothing(t *testing.T) {
	var l Layout
	for _, frame := range []string{
		"ID;", "AI;", "AI0;", "MC;", "MC003;", "MR0007;", "EX0020000;", "FV;", "TY;",
	} {
		if l.AllowedCommand([]byte(frame)) {
			t.Errorf("a zero Layout ADMITTED %q on behalf of no radio at all", frame)
		}
	}
	if l.AllowedCommand(mwFrame590(t)) {
		t.Error("a zero Layout ADMITTED a 50-byte MW")
	}
}

// TestAllowedCommand_AdmitsOnlyFramesTheEnvelopeAlsoAdmits. The per-command
// grammars sit IN FRONT OF the envelope both books print (framing.go's
// envelopeAllows), never instead of it, so every frame the grammars admit
// must satisfy the envelope too. If it ever did not, the layout-bearing
// framing's Allow — which is the conjunction — would be refusing its own
// builders' output, and this is where that is caught rather than at a
// session.
func TestAllowedCommand_AdmitsOnlyFramesTheEnvelopeAlsoAdmits(t *testing.T) {
	checked := 0
	for _, l := range []Layout{layout590SG(), layout590S(), layout480()} {
		for _, frame := range everyBuiltFrame(t, l) {
			checked++
			if !l.AllowedCommand(frame) {
				t.Errorf("%s: the gate refused its own builder's frame %q", l.Model(), frame)
			}
			if !envelopeAllows(frame) {
				t.Errorf("%s: the gate admits %q, which the documented envelope refuses — the grammars are meant to be strictly narrower", l.Model(), frame)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no frame reached this property, which therefore proved nothing")
	}
}

// everyBuiltFrame returns one frame from every builder l has, so a property
// over "what this codec emits" is written once rather than per test.
func everyBuiltFrame(t *testing.T, l Layout) [][]byte {
	t.Helper()
	var out [][]byte
	add := func(cmd Command, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: building a frame for the walk: %v", l.Model(), err)
		}
		out = append(out, cmd.Bytes())
	}
	add(l.BuildIDRead())
	add(l.BuildAIRead())
	add(l.BuildAISetOff())
	add(l.BuildMCRead())
	add(l.BuildMCSet(mustSlot(t, l, 0, ScanHalfNone)))
	add(l.BuildMCSet(mustSlot(t, l, 99, ScanHalfNone)))
	add(l.BuildMRRead(mustSlot(t, l, 42, ScanHalfNone)))
	add(l.BuildMWSet(populatedRecord(mustSlot(t, l, 42, ScanHalfNone))))
	add(l.BuildEXRead(EXAddress{P1: 0}))
	add(l.BuildEXRead(EXAddress{P1: 60}))
	if l.Book() == Book590 {
		add(l.BuildFVRead())
		add(l.BuildMRRead(mustSlot(t, l, 105, ScanUpper)))
		add(l.BuildMWSet(populatedRecord(mustSlot(t, l, 105, ScanUpper))))
	} else {
		add(l.BuildTYRead())
	}
	return out
}

// TestNewFramingFor_PutsTheGrammarsInFrontOfTheEnvelope is the seam: an
// Engine built from a layout-bearing framing gates on the eight grammars,
// where one built from a book alone gates on the envelope.
//
// THE TWO CONSTRUCTORS ARE NOT INTERCHANGEABLE and this is the pin that
// says so. The erase shape of 590:1579-1581 satisfies the envelope — it is a
// well-formed frame, which is a true statement about what the book prints —
// and it is not a grammar this codec builds, so the layout-bearing gate
// refuses it and the book-only one does not.
func TestNewFramingFor_PutsTheGrammarsInFrontOfTheEnvelope(t *testing.T) {
	l := layout590SG()
	lf, err := NewFramingFor(l)
	if err != nil {
		t.Fatalf("NewFramingFor: %v", err)
	}
	bf, err := NewFraming(l.Book())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}

	for _, frame := range everyBuiltFrame(t, l) {
		if !lf.Allow(frame) {
			t.Errorf("the layout-bearing framing refused its own builder's frame %q", frame)
		}
	}

	// The 42-byte erase shape: envelope yes, grammar no.
	erase := append(append([]byte{}, mwFrame590(t)[:recNameOff]...), ';')
	if !bf.Allow(erase) {
		t.Errorf("the book-only framing refused %q; its gate is the ENVELOPE alone and that frame satisfies it", erase)
	}
	if lf.Allow(erase) {
		t.Errorf("the layout-bearing framing ADMITTED the erase shape %q, which no builder in this package produces (decision 8, A5)", erase)
	}

	// Everything else the adapter promises is unchanged by the widening.
	if !lf.IsRejection([]byte("?;")) {
		t.Error("the layout-bearing framing lost IsRejection")
	}
	if _, ok := lf.(transport.FatalFramer); !ok {
		t.Error("the layout-bearing framing is not a transport.FatalFramer, so E; and O; would stop closing the port")
	}
	if seq := lf.InitSequence(); len(seq) != 1 || string(seq[0].Bytes()) != initFrame {
		t.Errorf("the layout-bearing framing's InitSequence is %v, want the one AI0; frame", seq)
	}
}

// TestNewFramingFor_RefusesAnUnconfiguredLayout. A framing built from a zero
// Layout would gate for no radio; the constructor is where a session can
// still be refused cheaply, on NewFraming's own model.
func TestNewFramingFor_RefusesAnUnconfiguredLayout(t *testing.T) {
	var l Layout
	f, err := NewFramingFor(l)
	if err == nil {
		t.Fatal("NewFramingFor accepted a zero Layout")
	}
	if f != nil {
		t.Errorf("NewFramingFor returned a non-nil framing alongside its error: %v", f)
	}
	if !strings.Contains(err.Error(), "layout") {
		t.Errorf("the refusal reads %v, and it should name the layout", err)
	}
}

// TestExactlyOneTrailingSemicolon pins the predicate directly, including the
// two shapes a count-only or a suffix-only check would let through.
func TestExactlyOneTrailingSemicolon(t *testing.T) {
	for _, tc := range []struct {
		frame string
		want  bool
	}{
		{"ID;", true},
		{"ID", false},
		{";", true},
		{"ID;;", false},
		{"I;D;", false},
		{";ID;", false},
		{"", false},
	} {
		if got := exactlyOneTrailingSemicolon([]byte(tc.frame)); got != tc.want {
			t.Errorf("exactlyOneTrailingSemicolon(%q) = %v, want %v", tc.frame, got, tc.want)
		}
	}
}
