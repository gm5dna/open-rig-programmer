// SPDX-License-Identifier: GPL-3.0-or-later

// Package kwtest is core/kw's conformance suite for a kw.Layout, expressed
// entirely through the EXPORTED API.
//
// WHY IT EXISTS. core/kw holds in-package walks that are the real safety
// property of this programme — every frame a layout's builders emit is well
// formed, is admitted by that layout's OWN outbound gate, and is refused by
// a layout describing another radio — and they drive fixtures declared in a
// _test.go file. A per-model package such as core/kw/ts590 imports core/kw,
// so it can never reach those fixtures: the import would be a cycle.
// Without this package a new Kenwood row would arrive with no way to run the
// properties that matter, and "it compiles and its own unit tests pass"
// would be the whole of its evidence.
//
// So the subset of those properties that can be stated through exported
// identifiers lives here, as a function any package can call:
//
//	func TestConformance(t *testing.T) { kwtest.Run(t, ts590.LayoutSG()) }
//
// It is a SUBSET, deliberately: several in-package checks read unexported
// helpers and cannot be restated. What this package can state, it states in
// full, and it COUNTS what it does so a layout that quietly contributes
// nothing fails loudly rather than passing in silence.
//
// WHY IT IS A NON-TEST FILE, AND WHY IT DOES NOT IMPORT "testing". The suite
// must be importable by ANOTHER package's tests, and a _test.go file is
// importable by nobody — so this is the deliberate net/http/httptest shape,
// and core/civ/civtest's exactly. Unlike httptest it imports only "sort",
// "strings" and core/kw: the T interface below is what keeps "testing" out,
// and it earns its keep twice over, because it is also the only way Run's
// refusal of an unconfigured layout can itself be tested. Nothing in the
// production tree imports this package and nothing should; its only callers
// are _test.go files.
//
// WHY Run REFUSES THE ZERO LAYOUT INSTEAD OF TESTING IT. "A zero Layout
// refuses everything" is a property this suite owns, but it is NOT what Run
// does when handed one. Run t.Fatal's, and RunZeroValue is a separate
// exported entry point. The reason is the vacuity trap: a model package
// whose exported layout was never initialised — a failed init, a typo
// selecting the wrong var — would call Run and, if Run silently switched to
// the refusal suite, receive a green conformance report for a radio it
// cannot describe. That is precisely the class of silent pass every walk in
// core/kw is written to prevent, so Run says what happened and names the
// other entry point instead.
package kwtest

import (
	"sort"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// T is the subset of *testing.T this suite uses. A *testing.T satisfies it,
// so callers write kwtest.Run(t, l) exactly as they would against a
// *testing.T parameter.
//
// IT IS AN INTERFACE FOR ONE REASON, and it is civtest's: the vacuity trap
// this suite exists to close — Run REFUSING an unconfigured layout rather
// than quietly running the refusal suite over it — is the single most
// important thing about Run's contract, and with a concrete *testing.T there
// is no way to test it. testing.TB cannot be implemented outside the testing
// package (it has an unexported method), and a real t.Fatal fails the very
// test asserting the refusal. A five-method interface makes the property
// provable and costs callers nothing.
type T interface {
	Helper()
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Logf(format string, args ...any)
}

// minConformanceFrames is the floor on the total frame count for one layout.
// A radio this programme can describe has a slot space and at least the
// eight grammars, so it produces dozens; the floor is set well below that
// because its job is to catch a walk that has stopped reaching the builders
// at all, not to police how many channels a model has.
const minConformanceFrames = 12

// maxSlotSamples bounds how many channels Run walks per declared slot range.
// A TS-590SG has 120 slots and a suite that walked every one of them would
// spend its time proving the same property a hundred and twenty times.
const maxSlotSamples = 5

// conformanceMenu is the menu address every layout is asked to build a read
// for. It is 000, which is a real address on all three registry rows —
// Display brightness on the TS-590S (590:569) and on the TS-480 (480:427),
// Firmware Version (read only) on the TS-590SG (590:749) — but this suite
// makes no claim about WHAT it is: membership is the per-row inventory's
// business (EXAddress, ex.go) and a conformance suite that consulted an
// inventory would need one, which is the import this package cannot have.
const conformanceMenu = 0

// Run holds l to every conformance property this package can state through
// core/kw's exported API, and fails t with a self-describing message for
// each violation.
//
// Every helper here calls t.Helper(), so a failure is reported at the
// caller's Run(t, l) line rather than inside this package: the message
// carries the detail, and the line number that matters to a model package's
// author is their own.
//
// It is an ERROR to call Run with an unconfigured layout — see the package
// doc comment for why that is a fatal misuse rather than a silent switch to
// RunZeroValue.
func Run(t T, l kw.Layout) {
	t.Helper()

	if !l.Configured() {
		t.Fatal("kwtest.Run was given an UNCONFIGURED layout (Configured() == false). " +
			"This is a misuse, not a conformance failure: an uninitialised package-level var reaches here " +
			"looking exactly like a radio, and reporting PASS for it is the silent green this suite exists " +
			"to prevent. If you meant to check the zero value's refusals, call kwtest.RunZeroValue(t).")
	}

	r := &run{t: t, l: l, frames: map[string]int{}, refusals: map[string]int{}, classes: map[kw.SlotClass]int{}}

	r.checkLayoutSelfConsistency()
	r.checkIdentity()
	r.checkAI()
	r.checkMC()
	r.checkMemoryReads()
	r.checkMemorySets()
	r.checkEX()
	r.checkGateRefusesTheUnacceptable()
	r.checkGateRefusesAMutatedPrintedFixedByte()
	r.checkNonVacuity()
}

// RunZeroValue holds the ZERO kw.Layout to the property every site in
// core/kw is written around: it describes no radio, so it builds nothing,
// parses nothing and admits nothing.
//
// It takes no layout, deliberately. A caller cannot hand it "their" zero
// layout and be told something about their own package: there is only one
// zero value and this is a property of the type.
func RunZeroValue(t T) {
	t.Helper()
	var l kw.Layout

	if l.Configured() {
		t.Fatal("the zero kw.Layout reports Configured() — every refusal in core/kw hangs off that predicate")
	}

	builders := []struct {
		what  string
		build func() (kw.Command, error)
	}{
		{"ID read", l.BuildIDRead},
		{"FV read", l.BuildFVRead},
		{"TY read", l.BuildTYRead},
		{"AI read", l.BuildAIRead},
		{"AI set", l.BuildAISetOff},
		{"MC read", l.BuildMCRead},
		{"MC set", func() (kw.Command, error) { return l.BuildMCSet(kw.Slot{}) }},
		{"MR read", func() (kw.Command, error) { return l.BuildMRRead(kw.Slot{}) }},
		{"MW set", func() (kw.Command, error) { return l.BuildMWSet(kw.Record{}) }},
		{"EX read", func() (kw.Command, error) { return l.BuildEXRead(kw.EXAddress{}) }},
	}
	for _, b := range builders {
		cmd, err := b.build()
		if err == nil {
			t.Errorf("the zero kw.Layout built a %s frame %q on behalf of no radio", b.what, cmd.Bytes())
		}
		if !cmd.IsZero() {
			t.Errorf("the zero kw.Layout returned a non-zero Command from %s alongside its error", b.what)
		}
	}

	// The gate, on the frames that consult no layout datum at all and would
	// otherwise pass a class-aware check vacuously.
	for _, frame := range []string{"ID;", "AI;", "AI0;", "MC;", "MC003;", "MR0007;", "EX0000000;", "FV;", "TY;"} {
		if l.AllowedCommand([]byte(frame)) {
			t.Errorf("the zero kw.Layout's gate ADMITTED %q on behalf of no radio", frame)
		}
	}

	if _, err := kw.NewFramingFor(l); err == nil {
		t.Errorf("kw.NewFramingFor accepted the zero Layout, so a session could open with a gate speaking for no radio")
	}
}

// run is one Run's state: the layout under test, plus the counters that make
// every property non-vacuous.
type run struct {
	t T
	l kw.Layout

	// frames counts built-and-checked frames per builder; refusals counts
	// refusals a check needed to SEE, per kind. A silent skip and an
	// enforced rule are indistinguishable without the second map.
	frames   map[string]int
	refusals map[string]int
	total    int

	// classes counts the slot classes actually reached, so a layout whose
	// second class was never sampled fails rather than passing on the
	// strength of its first.
	classes map[kw.SlotClass]int

	// roundTrips counts records that survived build -> parse intact.
	roundTrips int
}

func (r *run) name() string { return r.l.Model() }

// checkFrame is the property the whole suite is built around: this frame is
// well formed on the wire under the envelope both books print, fits this
// family's own bound, and this LAYOUT'S OWN gate admits it.
//
// A builder and a gate that disagree are worse than either being wrong
// alone: it means the programme cannot send a command it believes is valid,
// or the gate is not checking what the builder emits.
func (r *run) checkFrame(what string, frame []byte) {
	r.t.Helper()

	r.frames[what]++
	r.total++

	switch {
	case len(frame) < 3:
		r.t.Errorf("%s: %s produced %q, shorter than the smallest legal frame (two name bytes and a terminator: 590:12-13, 480:76)", r.name(), what, frame)
		return
	case frame[len(frame)-1] != ';':
		r.t.Errorf("%s: %s produced %q, whose last byte is not the ';' terminator (590:87-91, 480:113-118)", r.name(), what, frame)
		return
	case len(frame) > kw.DefaultMaxFrame:
		r.t.Errorf("%s: %s produced %d bytes, past this family's own %d-byte bound — its own accumulator would discard the exchange as contamination", r.name(), what, len(frame), kw.DefaultMaxFrame)
	}
	if n := strings.Count(string(frame), ";"); n != 1 {
		r.t.Errorf("%s: %s produced %q, which carries %d terminators; exactly one command must reach the wire per call", r.name(), what, frame, n)
	}
	for i, b := range frame[:len(frame)-1] {
		if b < 0x20 || b > 0x7e {
			r.t.Errorf("%s: %s produced %q, whose byte %d is %#02x — the 480 forbids the control codes generally (480:127-129) and A2's charset claim is bounded at 0x7E", r.name(), what, frame, i+1, b)
			break
		}
		if i < 2 && (b < 'A' || b > 'Z') {
			r.t.Errorf("%s: %s produced %q, whose command name is not two upper-case bytes (590:12-13, 480:76)", r.name(), what, frame)
			break
		}
	}
	if !r.l.AllowedCommand(frame) {
		r.t.Errorf("%s: its own gate REFUSED the %s frame %q — a builder and a gate that disagree mean this layout cannot send a command it believes is valid", r.name(), what, frame)
	}
}

// refuse records that a frame the gate MUST refuse was in fact refused, and
// reports it when it was not. The counter is what makes the refusal legs
// non-vacuous.
func (r *run) refuse(kind, why string, frame []byte) {
	r.t.Helper()
	if r.l.AllowedCommand(frame) {
		r.t.Errorf("%s: its gate ADMITTED %q — %s", r.name(), frame, why)
		return
	}
	r.refusals[kind]++
}

// checkLayoutSelfConsistency holds the layout's own accessors to the
// invariants NewLayout promises, so a hand-built Layout (or a future
// constructor bug) cannot reach the rest of this suite.
func (r *run) checkLayoutSelfConsistency() {
	r.t.Helper()
	l := r.l

	if l.Model() == "" {
		r.t.Fatal("a configured layout with an empty Model: every refusal it produces names the row it speaks for")
	}
	switch l.Book() {
	case kw.Book590, kw.Book480:
	default:
		r.t.Errorf("%s: Book is %v — a layout that names no document cannot quote a cause sentence (E13)", r.name(), l.Book())
	}
	if l.P2Policy() == kw.P2Unset {
		r.t.Errorf("%s: byte 4's policy is unset", r.name())
	}
	if l.Byte19() == kw.Byte19Unset {
		r.t.Errorf("%s: byte 19's meaning is unset", r.name())
	}
	if l.Byte28() == kw.Byte28Unset {
		r.t.Errorf("%s: byte 28's policy is unset", r.name())
	}
	if l.Byte3940() == kw.Byte3940Unset {
		r.t.Errorf("%s: bytes 39-40's meaning is unset", r.name())
	}
	if l.Byte41() == kw.Byte41Unset {
		r.t.Errorf("%s: byte 41's meaning is unset", r.name())
	}
	if l.ToneModes() == kw.ToneModesUnset {
		r.t.Errorf("%s: the tone-mode value set is unset", r.name())
	}
	if len(l.ModeNames()) == 0 {
		r.t.Fatalf("%s: the mode legend is empty, so no channel's mode could be named", r.name())
	}
	if len(l.Slots()) == 0 {
		r.t.Fatalf("%s: the slot space is empty, so no channel could be read or written", r.name())
	}
	// AN EMPTY SET IS NO LONGER A FAULT, since the TS-2000 lift (core/kw's
	// P10Policy/P12Policy/P13Policy plus Byte28Reverse/Byte41MemoryGroup):
	// a row that carries all six cross-checked positions live has nothing
	// to hard-wire at all — core/kw/ts2000/layout.go's own `PrintedFixed:
	// nil`. What NewLayout still enforces is the crossCheck consistency
	// between each axis and the set, which this suite's other legs
	// exercise through BuildMWSet/ParseMRAnswer; there is no longer a
	// standalone "the set must be non-empty" invariant to pin here.

	// The accessors must COPY. A layout a model package minted at
	// initialisation and handed out must not be editable by whoever holds
	// what it returned, or one caller's mutation would change every later
	// caller's radio.
	if names := l.ModeNames(); len(names) > 0 {
		for m := range names {
			delete(names, m)
			break
		}
		if len(l.ModeNames()) == len(names) {
			r.t.Errorf("%s: ModeNames() hands out the layout's own map — a caller could delete a mode from the radio", r.name())
		}
	}
	if slots := l.Slots(); len(slots) > 0 {
		slots[0].Hi = -1
		if l.Slots()[0].Hi == -1 {
			r.t.Errorf("%s: Slots() hands out the layout's own slice — a caller could empty the radio's slot space", r.name())
		}
	}
	if fixed := l.PrintedFixed(); len(fixed) > 0 {
		fixed[0].Printed = "!"
		if l.PrintedFixed()[0].Printed == "!" {
			r.t.Errorf("%s: PrintedFixed() hands out the layout's own slice", r.name())
		}
	}
}

// checkIdentity walks ID on every row and the row's OWN one of FV and TY,
// and requires the other book's command to be refused — the pin that keeps
// a layout from emitting a frame its radio's document does not print.
func (r *run) checkIdentity() {
	r.t.Helper()
	l := r.l

	cmd, err := l.BuildIDRead()
	if err != nil {
		r.t.Errorf("%s: BuildIDRead: %v", r.name(), err)
	} else {
		r.checkFrame("ID read", cmd.Bytes())
		// Bytes() must hand out a fresh copy every call: the TOCTOU closure
		// the Command type exists for. A caller mutating what the gate
		// approved, before the port write, is what it prevents.
		a, b := cmd.Bytes(), cmd.Bytes()
		if len(a) > 0 && &a[0] == &b[0] {
			r.t.Errorf("%s: two Command.Bytes() calls returned the same backing array", r.name())
		}
		a[0] = 'X'
		if c := cmd.Bytes(); c[0] != 'I' {
			r.t.Errorf("%s: mutating a returned Command.Bytes() changed the Command", r.name())
		}
		if cmd.String() == "" {
			r.t.Errorf("%s: the built Command's String() is empty — it is what a diagnostic line prints", r.name())
		}
	}

	// The ANSWER: six bytes, three digits, and never admitted outbound.
	answer := []byte("ID021;")
	if token, err := l.ParseIDAnswer(answer); err != nil {
		r.t.Errorf("%s: ParseIDAnswer rejected the six-byte, three-digit answer both books print (590:1119, 480:687): %v", r.name(), err)
	} else if token != "021" {
		r.t.Errorf("%s: ParseIDAnswer(%q) returned %q, want the three printed digits", r.name(), answer, token)
	}
	r.refuse("answer frame", "an ID ANSWER is never a legal outbound command, and admitting one lets a captured reply be written back", answer)
	r.refuse("Yaesu identity answer", "a Kenwood ID answer is 6 bytes with 3 digits and a Yaesu one is 7 with 4", []byte("ID0800;"))

	switch l.Book() {
	case kw.Book590:
		if cmd, err := l.BuildFVRead(); err != nil {
			r.t.Errorf("%s: BuildFVRead: %v", r.name(), err)
		} else {
			r.checkFrame("FV read", cmd.Bytes())
		}
		if v, err := l.ParseFVAnswer([]byte("FV1.00;")); err != nil {
			r.t.Errorf("%s: ParseFVAnswer rejected the book's own worked example (590:1035): %v", r.name(), err)
		} else if v != "1.00" {
			r.t.Errorf("%s: ParseFVAnswer returned %q, want %q", r.name(), v, "1.00")
		}
		if _, err := l.BuildTYRead(); err == nil {
			r.t.Errorf("%s: built a TY read, and TY appears nowhere in the TS-590S/SG document", r.name())
		} else {
			r.refusals["the other book's command"]++
		}
		r.refuse("the other book's command", "TY appears nowhere in the TS-590S/SG document", []byte("TY;"))
		r.refuse("answer frame", "an FV ANSWER is never a legal outbound command", []byte("FV1.00;"))
	case kw.Book480:
		if cmd, err := l.BuildTYRead(); err != nil {
			r.t.Errorf("%s: BuildTYRead: %v", r.name(), err)
		} else {
			r.checkFrame("TY read", cmd.Bytes())
		}
		if a, err := l.ParseTYAnswer([]byte("TY001;")); err != nil {
			r.t.Errorf("%s: ParseTYAnswer rejected a printed variant (480:1627): %v", r.name(), err)
		} else if a.Variant != '1' {
			r.t.Errorf("%s: ParseTYAnswer returned variant %q, want '1'", r.name(), a.Variant)
		}
		if _, err := l.ParseTYAnswer([]byte("TY004;")); err == nil {
			r.t.Errorf("%s: ParseTYAnswer accepted P2 '4'; decision 4 refuses a fifth variant rather than reporting it opaquely", r.name())
		} else {
			r.refusals["an unprinted TY variant"]++
		}
		if _, err := l.BuildFVRead(); err == nil {
			r.t.Errorf("%s: built an FV read, and FV appears nowhere in the 2003 TS-480 document", r.name())
		} else {
			r.refusals["the other book's command"]++
		}
		r.refuse("the other book's command", "FV appears nowhere in the 2003 TS-480 document", []byte("FV;"))
		r.refuse("answer frame", "a TY ANSWER is never a legal outbound command", []byte("TY001;"))
	default:
		// checkLayoutSelfConsistency has already reported an unknown Book,
		// and kw.NewLayout refuses one, so this arm is unreachable today.
		// It is here because a silent arm would drop the WHOLE FV/TY family
		// from the suite on the day a third book is added — the sibling
		// switch above refuses rather than falls through, and so does this.
		r.t.Errorf("%s: Book is %v, so neither the FV leg nor the TY leg ran and this whole command family went unchecked", r.name(), l.Book())
	}
}

// checkAI walks the read and the one Set, and requires every other AI state
// to be refused — including the two the OTHER book prints, which is what
// stops a per-book widening being read as a family one.
func (r *run) checkAI() {
	r.t.Helper()

	if cmd, err := r.l.BuildAIRead(); err != nil {
		r.t.Errorf("%s: BuildAIRead: %v", r.name(), err)
	} else {
		r.checkFrame("AI read", cmd.Bytes())
	}
	if cmd, err := r.l.BuildAISetOff(); err != nil {
		r.t.Errorf("%s: BuildAISetOff: %v", r.name(), err)
	} else {
		r.checkFrame("AI set", cmd.Bytes())
		if got := string(cmd.Bytes()); got != "AI0;" {
			r.t.Errorf("%s: the one AI Set this codec builds is %q, want %q", r.name(), got, "AI0;")
		}
	}
	for _, state := range []string{"AI1;", "AI2;", "AI3;", "AI4;", "AI5;", "AI9;"} {
		r.refuse("an AI state other than OFF", "every non-zero AI value in either book's legend turns Auto Information ON, pushing frames nobody asked for into a session that correlates answers by prefix", []byte(state))
	}
}

// checkMC walks the read and a Set for every sampled ORDINARY MEMORY slot,
// round-trips each Set through the answer decoder, and requires a Set naming
// any other class to be refused by BOTH the builder and the gate (A16).
func (r *run) checkMC() {
	r.t.Helper()
	l := r.l

	if cmd, err := l.BuildMCRead(); err != nil {
		r.t.Errorf("%s: BuildMCRead: %v", r.name(), err)
	} else {
		r.checkFrame("MC read", cmd.Bytes())
	}

	for _, s := range r.sampleSlots() {
		cmd, err := l.BuildMCSet(s)
		if s.Class() != kw.SlotMemory {
			if err == nil {
				r.t.Errorf("%s: BuildMCSet built a recall of %v, which is %v — A16 narrows the MC Set domain to ordinary memory", r.name(), s, s.Class())
				continue
			}
			r.refusals["an MC Set outside ordinary memory"]++
			// And the gate must refuse the same frame, arriving from
			// anywhere: the Set and the Answer are one wire shape.
			r.refuse("an MC Set outside ordinary memory", "A16 narrows the MC Set domain to ordinary memory, and MC's Set and Answer share six bytes exactly", mcFrame(s.Number()))
			continue
		}
		if err != nil {
			r.t.Errorf("%s: BuildMCSet(%v): %v", r.name(), s, err)
			continue
		}
		r.checkFrame("MC set", cmd.Bytes())
		ch, err := l.ParseMCAnswer(cmd.Bytes())
		if err != nil {
			r.t.Errorf("%s: ParseMCAnswer refused this layout's own MC Set %q: the two share one wire shape exactly (%v)", r.name(), cmd.Bytes(), err)
			continue
		}
		if ch.Number != s.Number() {
			r.t.Errorf("%s: MC Set for %v decoded as channel %d", r.name(), s, ch.Number)
		}
		if ch.Class != s.Class() {
			r.t.Errorf("%s: MC Set for %v decoded with class %v", r.name(), s, ch.Class)
		}
		r.roundTrips++
	}
}

// mcFrame renders an MC Set naming number with a zero hundreds digit. It is
// a TEST-SIDE renderer deliberately: the point of the refusals it feeds is
// that a frame arriving from anywhere — not only from this package's own
// builder, which would not have produced it — is refused.
func mcFrame(number int) []byte {
	return []byte{'M', 'C', byte('0' + number/100%10), byte('0' + number/10%10), byte('0' + number%10), ';'}
}

// checkMemoryReads walks an MR read for every sampled slot and requires the
// 50-byte MR ANSWER to be refused outbound.
func (r *run) checkMemoryReads() {
	r.t.Helper()

	for _, s := range r.sampleSlots() {
		cmd, err := r.l.BuildMRRead(s)
		if err != nil {
			r.t.Errorf("%s: BuildMRRead(%v): %v", r.name(), s, err)
			continue
		}
		r.checkFrame("MR read", cmd.Bytes())
		r.classes[s.Class()]++
	}

	// A slot that was never resolved against a layout, and one resolved
	// against a DIFFERENT slot space: both are values a caller can hold.
	if _, err := r.l.BuildMRRead(kw.Slot{}); err == nil {
		r.t.Errorf("%s: BuildMRRead accepted a slot that was never resolved against a layout", r.name())
	} else {
		r.refusals["an unresolved slot"]++
	}
	if _, err := r.l.NewSlot(r.highestSlot()+1, kw.ScanHalfNone); err == nil {
		r.t.Errorf("%s: NewSlot resolved %d, one past the top of its own declared slot space", r.name(), r.highestSlot()+1)
	} else {
		r.refusals["a slot outside this layout's space"]++
	}
}

// checkMemorySets builds an MW for every sampled slot, round-trips it
// through the record decoder, and requires the ANSWER form of the same fifty
// bytes to be refused outbound.
func (r *run) checkMemorySets() {
	r.t.Helper()
	l := r.l

	// The witness slot the matcher legs below correlate AGAINST: any slot
	// of this row's own space that is not the one being answered.
	witness := r.firstMemorySlot()

	for _, s := range r.sampleSlots() {
		rec := r.conformanceRecord(s)
		cmd, err := l.BuildMWSet(rec)
		if err != nil {
			r.t.Errorf("%s: BuildMWSet(%v): %v", r.name(), s, err)
			continue
		}
		frame := cmd.Bytes()
		r.checkFrame("MW set", frame)
		if len(frame) != kw.RecordLen {
			r.t.Errorf("%s: BuildMWSet produced %d bytes, want exactly %d — the short form of 590:1579-1581 ERASES the channel", r.name(), len(frame), kw.RecordLen)
		}

		// The same fifty bytes under the MR prefix are the ANSWER, which is
		// what a radio sends; decoding it back must reproduce the record.
		answer := append([]byte{}, frame...)
		answer[0], answer[1] = 'M', 'R'
		got, err := l.ParseMRAnswer(answer)
		if err != nil {
			r.t.Errorf("%s: ParseMRAnswer refused the fifty bytes this layout's own MW builder produced (%q): %v", r.name(), answer, err)
			continue
		}
		if got.FreqHz != rec.FreqHz || got.Mode != rec.Mode || got.Name != rec.Name ||
			got.Byte19 != rec.Byte19 || got.Byte28 != rec.Byte28 || got.Byte41 != rec.Byte41 ||
			got.Byte3940 != rec.Byte3940 || got.ToneMode != rec.ToneMode ||
			got.ToneIndex != rec.ToneIndex || got.CTCSSIndex != rec.CTCSSIndex ||
			got.DCSCode != rec.DCSCode || got.Shift != rec.Shift || got.OffsetHz != rec.OffsetHz ||
			got.Slot.Number() != s.Number() || got.Slot.Class() != s.Class() || got.Slot.Half() != s.Half() {
			r.t.Errorf("%s: a record did not survive build -> parse for %v.\n  sent %+v\n  back %+v", r.name(), s, rec, got)
			continue
		}
		if got.Empty {
			r.t.Errorf("%s: a populated record for %v came back as the empty channel of 590:1492-1493", r.name(), s)
		}
		r.roundTrips++
		r.refuse("answer frame", "a 50-byte MR ANSWER is never a legal outbound command — MR has no Set on either radio (480:911, erratum E17)", answer)

		// THE ANSWER-TO-READ CORRELATION, which for MR is the matcher's
		// and no caller's. Every memory answer is fifty bytes and starts
		// "MR", so the channel number in P2/P3 is the only thing that
		// separates one from another; a late answer for a channel whose
		// read has already timed out is otherwise correlated to the NEXT
		// read (kw.Layout.MRAnswerMatcher's own doc comment).
		if !l.MRAnswerMatcher(s)(answer) {
			r.t.Errorf("%s: the MR matcher for %v refused the answer for that very slot %q", r.name(), s, answer)
		}
		if s.Number() != witness.Number() {
			if l.MRAnswerMatcher(witness)(answer) {
				r.t.Errorf("%s: the MR matcher for %v correlated %v's answer %q — one channel's record would be returned under another's number", r.name(), witness, s, answer)
			} else {
				r.refusals["another channel's MR answer"]++
			}
		}
	}

	// The EMPTY record, which is never written: the only documented clear is
	// the short MW of 590:1579-1581, which this milestone never builds.
	empty := kw.Record{Slot: r.firstMemorySlot(), Empty: true}
	if _, err := l.BuildMWSet(empty); err == nil {
		r.t.Errorf("%s: BuildMWSet wrote an EMPTY channel; the only documented clear is the short MW of 590:1579-1581 (decision 8, A5)", r.name())
	} else {
		r.refusals["an empty record"]++
	}

	r.checkEmptyChannel()
}

// checkEmptyChannel holds the row to BOTH halves of one sentence: "If the
// selected channel is empty, P4 ~ P15 will be 0 and P16 will be blank."
// (590:1492-1493). The first half is A18a and is why an empty channel is
// never refused; the second is A3, whose reading of "blank" — eight spaces —
// this codec states and enforces rather than admitting whatever P16 carries
// under a window it has already called empty.
//
// THE FRAME IS BUILT FROM THE BOOKS' OWN POSITIONS, not from core/kw's
// offsets: positions 7-41 are P4 ~ P15 and positions 42-49 are P16 on both
// charts (590:1440-1461, 480:923-943), and writing them here keeps the
// suite's frame independent of the codec it is checking.
//
// IT MAKES NO CLAIM THAT ANY RADIO SENDS ONE. Whether a TS-480 answers an
// empty channel at all is A4, a question about that radio settled at
// hardware item 3; the shape is a property of the FRAME and is held on every
// row.
func (r *run) checkEmptyChannel() {
	r.t.Helper()
	l := r.l

	s := r.firstMemorySlot()
	cmd, err := l.BuildMWSet(r.conformanceRecord(s))
	if err != nil {
		r.t.Errorf("%s: BuildMWSet(%v): %v", r.name(), s, err)
		return
	}
	empty := cmd.Bytes()
	empty[0], empty[1] = 'M', 'R'
	for i := 6; i <= 40; i++ {
		empty[i] = '0'
	}
	for i := 41; i <= 48; i++ {
		empty[i] = ' '
	}

	rec, err := l.ParseMRAnswer(empty)
	if err != nil {
		r.t.Errorf("%s: ParseMRAnswer refused the empty channel of 590:1492-1493 (A18a) %q: %v", r.name(), empty, err)
		return
	}
	if !rec.Empty || rec.Name != "" {
		r.t.Errorf("%s: the empty channel %q decoded as Empty = %v, Name = %q", r.name(), empty, rec.Empty, rec.Name)
	}

	named := append([]byte{}, empty...)
	copy(named[41:], "NAME")
	if got, err := l.ParseMRAnswer(named); err == nil {
		r.t.Errorf("%s: ParseMRAnswer accepted an empty window whose P16 is not blank, returning Name = %q — the same sentence says P16 \"will be blank\", and A3 reads blank as eight spaces", r.name(), got.Name)
	} else {
		r.refusals["an empty channel whose P16 is not blank"]++
	}
}

// checkEX walks the ten-byte read, requires the ANSWER shape — the same
// frame with P5 appended — to be refused outbound, and holds the row to its
// OWN printed menu domain in both directions.
func (r *run) checkEX() {
	r.t.Helper()
	l := r.l

	cmd, err := l.BuildEXRead(kw.EXAddress{P1: conformanceMenu})
	if err != nil {
		r.t.Errorf("%s: BuildEXRead(%03d): %v", r.name(), conformanceMenu, err)
		return
	}
	read := cmd.Bytes()
	r.checkFrame("EX read", read)
	if len(read) != kw.EXReadLen {
		r.t.Errorf("%s: BuildEXRead produced %d bytes, want exactly %d (590:552, 480:410)", r.name(), len(read), kw.EXReadLen)
	}

	// The answer: the read's own ten bytes with a one-character P5 inserted
	// before the terminator, which is the shape both books print.
	answer := append(append([]byte{}, read[:len(read)-1]...), '3', ';')
	item := kw.EXItem{Addr: kw.EXAddress{P1: conformanceMenu}, Name: "the conformance menu", Digits: 1}
	if v, err := l.ParseEXAnswer(answer, item); err != nil {
		r.t.Errorf("%s: ParseEXAnswer refused the answer to its own read %q: %v", r.name(), answer, err)
	} else if v != "3" {
		r.t.Errorf("%s: ParseEXAnswer(%q) returned %q, want %q", r.name(), answer, v, "3")
	}
	r.refuse("answer frame", "EX's Set and Answer are one wire shape, so admitting it would let a captured menu reply be written back", answer)

	// A19's ceiling: an answer wider than the inventory row's printed width.
	wide := append(append([]byte{}, read[:len(read)-1]...), '3', '3', ';')
	if _, err := l.ParseEXAnswer(wide, item); err == nil {
		r.t.Errorf("%s: ParseEXAnswer accepted a P5 wider than the printed width (A19)", r.name())
	} else {
		r.refusals["an EX answer past its printed width"]++
	}

	// A different address's answer, which is the full-address obligation.
	other := append([]byte{}, answer...)
	other[4] = '9'
	if _, err := l.ParseEXAnswer(other, item); err == nil {
		r.t.Errorf("%s: ParseEXAnswer accepted another menu's answer as this read's; the whole address is the correlation key", r.name())
	} else {
		r.refusals["another address's EX answer"]++
	}

	r.checkEXDomain()
}

// checkEXDomain holds the row to the menu domain ITS OWN BOOK PRINTS: 000 ~
// 087 on the TS-590S (590:543), 000 ~ 099 on the TS-590SG (590:544) and 000
// ~ 060 on the TS-480 (480:401).
//
// IT IS THE ONE LEG THAT SEPARATES THE TWO 590 ROWS ON A READ FRAME. Their
// grids, legends and hard-wired bytes are one book's, so a suite that never
// asked for the last printed address would pass on either row's bound put on
// the other — which is exactly the mistake a driver sweeping the menu
// surface would make, the two inventories being one identifier apart.
func (r *run) checkEXDomain() {
	r.t.Helper()
	l := r.l

	// The last address this row prints must build and pass its own gate.
	last := l.MaxEXAddress()
	cmd, err := l.BuildEXRead(kw.EXAddress{P1: last})
	if err != nil {
		r.t.Errorf("%s: BuildEXRead refused menu %03d, the last address this row's own book prints: %v", r.name(), last, err)
		return
	}
	r.checkFrame("EX read", cmd.Bytes())

	if last == 255 {
		r.t.Errorf("%s: its printed EX menu domain reaches 255, the whole address space an EXAddress holds — this suite has no address above it to hold the gate to, so the domain leg would pass vacuously", r.name())
		return
	}

	// And the first address it does NOT print: refused by the builder, and
	// refused by the gate even when the frame is handed to it fully formed.
	if _, err := l.BuildEXRead(kw.EXAddress{P1: last + 1}); err == nil {
		r.t.Errorf("%s: BuildEXRead built a read of menu %03d, one past the domain this row's own book prints (000 ~ %03d)", r.name(), last+1, last)
	} else {
		r.refusals["an EX read past the row's printed menu domain"]++
	}
	beyond := []byte("EX" + threeDigits(int(last)+1) + "0000;")
	r.refuse("an EX read past the row's printed menu domain", "the menu domain is printed per row (590:543, 590:544, 480:401), so an address this row's book does not print is not one this row may be sent", beyond)

	// AND THE INGRESS DIRECTION, which the builder and the gate cannot
	// cover: the same ten bytes with a P5 are the ANSWER a radio sends, and
	// a menu number this row's book does not print is not a setting this
	// row has. Without this leg the parser would be the one unbounded path
	// through the domain.
	answer := append(append([]byte{}, beyond[:len(beyond)-1]...), '3', ';')
	item := kw.EXItem{Addr: kw.EXAddress{P1: last + 1}, Name: "a menu number this row's book does not print", Digits: 1}
	if v, err := l.ParseEXAnswer(answer, item); err == nil {
		r.t.Errorf("%s: ParseEXAnswer returned %q for menu %03d, past the domain this row's own book prints (000 ~ %03d)", r.name(), v, last+1, last)
	} else {
		r.refusals["an EX answer past the row's printed menu domain"]++
	}
}

// threeDigits renders n as the EX address field's three zero-padded digits.
// It is here rather than borrowed from core/kw because the frames this suite
// hands the gate must be built independently of the builders it is checking.
func threeDigits(n int) string {
	return string([]byte{byte('0' + n/100%10), byte('0' + n/10%10), byte('0' + n%10)})
}

// checkGateRefusesTheUnacceptable is the plan's negative-pin list, held
// against every layout rather than against one fixture.
func (r *run) checkGateRefusesTheUnacceptable() {
	r.t.Helper()

	for _, tc := range []struct{ frame, why string }{
		{"MT001;", "MT is a Yaesu command; neither Kenwood book has one, and the channel name is field P16 of MR/MW"},
		{"MT;", "the same, in its read shape"},
		{"MA;", "MA belongs to the 890S/990S family, which this milestone does not register"},
		{"XI;", "XI is neither built nor parsed here — A20's paste-error reading is unlifted"},
		{"XT;", "XT likewise"},
		{"IF;", "no transceiver-status read is built"},
		{"PS1;", "no discovery or wake-up frame of any kind is built (decision 5)"},
		{"ID;ID;", "two commands in one frame: a second terminator splits it on the radio's own parser"},
		{";ID;", "a leading terminator, which a suffix-only check would admit"},
		{"ID", "no terminator at all"},
		{"id;", "a lower-case opcode: the books' either-case permission is about what the radio ACCEPTS, not a licence to emit a second spelling"},
	} {
		r.refuse("an unbuilt command", tc.why, []byte(tc.frame))
	}

	// Every MW width but fifty, including the 42-byte erase shape of
	// 590:1579-1581 (A5), which this programme never builds (decision 8).
	for _, n := range []int{7, 42, 49, 51} {
		frame := make([]byte, n)
		copy(frame, "MW")
		for i := 2; i < n-1; i++ {
			frame[i] = '0'
		}
		frame[n-1] = ';'
		r.refuse("an MW of the wrong width", "both books print one 50-byte grid, and the SHORT MW of 590:1579-1581 ERASES the channel", frame)
	}
}

// checkGateRefusesAMutatedPrintedFixedByte is the leg that makes the MW
// admission non-vacuous: a frame of exactly the right width and prefix,
// differing from a real one in a single byte the book prints as a constant,
// must be refused. A gate that checked only the prefix and the terminator
// would admit every one of these.
func (r *run) checkGateRefusesAMutatedPrintedFixedByte() {
	r.t.Helper()

	cmd, err := r.l.BuildMWSet(r.conformanceRecord(r.firstMemorySlot()))
	if err != nil {
		r.t.Errorf("%s: BuildMWSet for the mutation leg: %v", r.name(), err)
		return
	}
	good := cmd.Bytes()
	if !r.l.AllowedCommand(good) {
		r.t.Errorf("%s: the unmutated frame is already refused, so every mutation below would prove nothing", r.name())
		return
	}

	// A row with no printed-fixed byte at all — every cross-checked
	// position traded for a live axis, core/kw/ts2000's own shape — has
	// nothing for this leg to mutate; that is not a fault (see
	// checkLayoutSelfConsistency's own comment), so the loop below is
	// simply empty and only the terminator leg beneath it runs.
	fixed := r.l.PrintedFixed()
	for _, ff := range fixed {
		for off := ff.Pos - 1; off < ff.Pos-1+len(ff.Printed); off++ {
			mutated := append([]byte{}, good...)
			// '0' is what every Kenwood printed-fixed run carries, so '1'
			// is a different byte at every one of them.
			if mutated[off] == '1' {
				mutated[off] = '0'
			} else {
				mutated[off] = '1'
			}
			r.refuse("a mutated printed-fixed byte", "position "+itoa(off+1)+" is hard-wired in this radio's own chart, and a frame carrying anything else is one no builder here produces", mutated)
		}
	}

	// And the terminator, which is the one byte no frame may vary.
	mutated := append([]byte{}, good...)
	mutated[len(mutated)-1] = '0'
	r.refuse("a missing terminator", "the terminator is the last byte of every frame both books print (590:87-91, 480:113-118)", mutated)
}

// checkNonVacuity is what turns "no failures" into "the properties ran".
func (r *run) checkNonVacuity() {
	r.t.Helper()

	if r.total < minConformanceFrames {
		r.t.Errorf("%s: only %d frames reached checkFrame, want at least %d — a walk that stopped reaching the builders reports no failures and proves nothing", r.name(), r.total, minConformanceFrames)
	}
	for _, what := range []string{"ID read", "AI read", "AI set", "MC read", "MC set", "MR read", "MW set", "EX read"} {
		if r.frames[what] == 0 {
			r.t.Errorf("%s: no %s frame was built and checked", r.name(), what)
		}
	}
	if r.roundTrips == 0 {
		r.t.Errorf("%s: no record survived a build -> parse round trip, so the codec was never exercised in both directions", r.name())
	}
	kinds := []string{"answer frame", "an AI state other than OFF", "an unbuilt command", "an MW of the wrong width", "an empty record", "an EX read past the row's printed menu domain", "an EX answer past the row's printed menu domain", "another channel's MR answer", "an empty channel whose P16 is not blank"}
	// "A MUTATED PRINTED-FIXED BYTE" IS REQUIRED ONLY OF A ROW THAT HAS ONE
	// TO MUTATE. checkGateRefusesAMutatedPrintedFixedByte's own mutation
	// loop is empty for a row with no hard-wired byte at all (every
	// cross-checked position traded for a live axis — core/kw/ts2000's
	// `PrintedFixed: nil`), so this refusal kind is not merely unseen
	// there, it is STRUCTURALLY IMPOSSIBLE: demanding it unconditionally
	// would fail a well-formed conformance run for a fact about the RADIO,
	// not a gap in the walk. checkLayoutSelfConsistency's own comment
	// records the same retired invariant.
	if len(r.l.PrintedFixed()) > 0 {
		kinds = append(kinds, "a mutated printed-fixed byte")
	}
	for _, kind := range kinds {
		if r.refusals[kind] == 0 {
			r.t.Errorf("%s: no refusal of kind %q was ever SEEN — a silent skip and an enforced rule are indistinguishable without this count", r.name(), kind)
		}
	}
	// Every class this layout DECLARES must have been sampled, or a row's
	// second class would pass on the strength of its first.
	for _, sr := range r.l.Slots() {
		if r.classes[sr.Class] == 0 {
			r.t.Errorf("%s: slot class %v is declared (%03d-%03d) and no channel of it was read", r.name(), sr.Class, sr.Lo, sr.Hi)
		}
	}
	r.t.Logf("%s: %d frames checked across %d builders, %d record round trips, %d refusal kinds seen", r.name(), r.total, len(r.frames), r.roundTrips, len(r.refusals))
}

// sampleSlots returns up to maxSlotSamples slots from every range this
// layout declares, in a deterministic order.
//
// A SECTION-DEFINED CHANNEL CONTRIBUTES BOTH HALVES, because they are two
// frames and not one: the lower half is the printed START frequency and the
// upper the END (590:1529-1531), and a suite that sampled only one would
// never build a frame with P1='1'.
func (r *run) sampleSlots() []kw.Slot {
	r.t.Helper()
	var out []kw.Slot
	ranges := r.l.Slots()
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].Lo < ranges[j].Lo })
	for _, sr := range ranges {
		for _, n := range sampleNumbers(sr.Lo, sr.Hi) {
			halves := []kw.ScanHalf{kw.ScanHalfNone}
			if sr.Class == kw.SlotScan {
				halves = []kw.ScanHalf{kw.ScanLower, kw.ScanUpper}
			}
			for _, h := range halves {
				s, err := r.l.NewSlot(n, h)
				if err != nil {
					r.t.Errorf("%s: NewSlot(%d, %v) refused a number inside its own declared range %03d-%03d: %v", r.name(), n, h, sr.Lo, sr.Hi, err)
					continue
				}
				out = append(out, s)
			}
		}
	}
	if len(out) == 0 {
		r.t.Fatalf("%s: no slot could be resolved from a declared slot space", r.name())
	}
	return out
}

// sampleNumbers picks up to maxSlotSamples numbers spread across lo..hi,
// always including both ends: a boundary is where an off-by-one lives.
func sampleNumbers(lo, hi int) []int {
	if hi < lo {
		return nil
	}
	n := hi - lo + 1
	if n <= maxSlotSamples {
		out := make([]int, 0, n)
		for i := lo; i <= hi; i++ {
			out = append(out, i)
		}
		return out
	}
	out := []int{lo, hi}
	step := n / (maxSlotSamples - 1)
	for i := lo + step; i < hi && len(out) < maxSlotSamples; i += step {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// firstMemorySlot is the lowest ordinary-memory slot this layout declares.
func (r *run) firstMemorySlot() kw.Slot {
	r.t.Helper()
	ranges := r.l.Slots()
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].Lo < ranges[j].Lo })
	for _, sr := range ranges {
		if sr.Class != kw.SlotMemory {
			continue
		}
		s, err := r.l.NewSlot(sr.Lo, kw.ScanHalfNone)
		if err != nil {
			r.t.Fatalf("%s: NewSlot(%d) refused the first channel of its own memory range: %v", r.name(), sr.Lo, err)
		}
		return s
	}
	r.t.Fatalf("%s: the layout declares no ordinary memory range, and every Kenwood row this design registers has one", r.name())
	return kw.Slot{}
}

// highestSlot is the top of this layout's declared slot space.
func (r *run) highestSlot() int {
	hi := -1
	for _, sr := range r.l.Slots() {
		if sr.Hi > hi {
			hi = sr.Hi
		}
	}
	return hi
}

// conformanceRecord is a populated channel every registered row can carry:
// 14.250 MHz, the row's own first mode by nibble, no tone, and the quiet
// printed value in every raw byte.
//
// EVERY RAW BYTE IS '0'/"00"/ZERO, WHICH IS LEGAL UNDER BOTH READINGS OF
// EACH AXIS — byte 19 is the data mode on the 590 pair and the lockout on
// the 480 and '0' is a printed value of both; byte 28 is FILTER A, a live
// REVERSE status, or a hard-wired constant, all of which admit '0'; bytes
// 39-40 are "FM Normal" or ST step index 0; byte 41 is the lockout, a live
// Memory Group (0-9, and 0 is a real group), or a constant. P10's DCS code
// and P13's offset frequency are both a printed "all zero" under
// P10FixedZero/P13FixedZero and a perfectly ordinary value (no DCS, no
// offset) under P10DCSCode/P13OffsetLive, so Record's own zero values for
// DCSCode/OffsetHz need no explicit field here. P12's Shift is the one
// exception: Go's zero byte is 0x00, not the ASCII '0' every reading of
// P12 requires (P12FixedZero's constant AND P12ShiftLive's Simplex), so it
// is the one raw byte this record sets explicitly for a reason. One record
// serves every row without this suite branching on an axis, and it
// therefore cannot silently stop exercising one.
func (r *run) conformanceRecord(s kw.Slot) kw.Record {
	r.t.Helper()
	return kw.Record{
		Slot:     s,
		FreqHz:   14_250_000,
		Mode:     r.firstMode(),
		Byte19:   '0',
		ToneMode: kw.ToneModeOff,
		Byte28:   '0',
		Byte3940: "00",
		Byte41:   '0',
		Shift:    '0',
		Name:     "KWTEST",
	}
}

// firstMode is the lowest nibble this layout's MD legend names, chosen by
// nibble rather than by map order so that a failure is reproducible.
func (r *run) firstMode() kw.Mode {
	r.t.Helper()
	names := r.l.ModeNames()
	if len(names) == 0 {
		r.t.Fatalf("%s: the mode legend is empty", r.name())
	}
	modes := make([]int, 0, len(names))
	for m := range names {
		modes = append(modes, int(m))
	}
	sort.Ints(modes)
	return kw.Mode(modes[0])
}

// itoa renders a small non-negative int without importing strconv, which
// would be the package's fourth import for one message.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
