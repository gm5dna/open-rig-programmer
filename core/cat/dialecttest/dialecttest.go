// SPDX-License-Identifier: GPL-3.0-or-later

// Package dialecttest is core/cat's conformance suite for a cat.Dialect,
// expressed entirely through the EXPORTED API.
//
// WHY IT EXISTS. core/cat holds two in-package walks that are the real
// safety property of this program — every frame a dialect's builders emit is
// clean, carries exactly one terminator, and is admitted by that dialect's
// OWN outbound gate (dialectgate_test.go), and each MT form's own builders
// contribute while the other form's are seen to refuse (mtcombined_test.go).
// Both drive allTestDialects(), an in-package fixture. A radio-model package
// such as core/cat/ftdx10 imports core/cat, so it can never reach that
// fixture: the import would be a cycle. Without this package a new model
// would arrive with no way to run the properties that matter, and "it
// compiles and its own unit tests pass" would be the whole of its evidence.
//
// So the subset of those properties that can be stated through exported
// identifiers lives here, as a function any package can call:
//
//	func TestConformance(t *testing.T) { dialecttest.Run(t, ftdx10.FTdx10) }
//
// It is a SUBSET, deliberately. Several in-package checks read unexported
// policy (TagMaxBytes, TagFill, PadByte) and cannot be restated here; those
// stay where they are. What this package can state, it states in full, and
// it counts what it does so a dialect that quietly contributes nothing fails
// loudly rather than passing in silence.
//
// WHY A LIBRARY PACKAGE IMPORTS "testing". This is a non-test file importing
// testing, which is normally a smell: it drags the testing flag set into any
// binary that links it. It is the deliberate net/http/httptest and
// testing/quick pattern, and it is the only shape that works here — the
// suite must be importable by ANOTHER package's tests, and a _test.go file
// is importable by nobody. Nothing in the production tree imports this
// package, and nothing should: its only callers are _test.go files.
//
// WHY Run REFUSES THE ZERO DIALECT INSTEAD OF TESTING IT. "A zero Dialect
// refuses everything" is a property this suite owns, but it is NOT what Run
// does when handed one. Run t.Fatal's, and RunZeroValue is a separate
// exported entry point. The reason is the vacuity trap: a model package
// whose exported dialect var was never initialised — a failed init, a typo
// selecting the wrong var — would call Run and, if Run silently switched to
// the refusal suite, receive a green conformance report for a radio it
// cannot describe. That is precisely the class of silent pass every walk in
// core/cat is written to prevent, so Run says what happened and names the
// other entry point instead.
package dialecttest

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

// conformanceTags are the logical tags Run offers a dialect's MT Set
// builders.
//
// Their LAST BYTES are all different ('A', 'Y', '7', '3'), and that is the
// load-bearing property, not the strings themselves. A dialect declares one
// fill byte and one pad byte, so at most two of these four can be spoiled by
// them: on the combined form a tag ending in the fill byte is REFUSED by the
// builder (it would not round-trip), and on the short form a tag ending in
// the pad byte builds but comes back trimmed. Either way at least two
// candidates survive on any dialect, which is what lets Run require an exact
// tag round trip without knowing a byte of the dialect's tag policy.
//
// The one-byte entries are here so the suite still has something to offer a
// dialect declaring TagMaxBytes 1, which V9 permits.
var conformanceTags = []string{"A", "XY", "7", "K3"}

// conformanceFreqHz is the frequency every memory record Run builds carries:
// 14.250 MHz, inside the 9-digit P2 field and nonzero, which is all any
// dialect's write validation asks of it.
const conformanceFreqHz = 14_250_000

// minConformanceFrames is the floor on the total frame count for one
// dialect. A radio this program can describe has a slot space, a mode table
// and a menu, so it produces hundreds; the floor is set well below that
// because its job is to catch a walk that has stopped reaching the builders
// at all, not to police how many channels a family has.
const minConformanceFrames = 20

// universallyRefusedFrames are frames NO dialect may admit, whatever it
// declares: an unknown command, a missing terminator, a doubled terminator,
// a frame too short to be one, and — the safety-critical one — a frame
// carrying a second command after an interior ';'.
//
// They are what stops "every built frame is admitted by its own gate" from
// being satisfied by a gate that returns true for everything.
var universallyRefusedFrames = [][]byte{
	nil,
	[]byte(""),
	[]byte("M"),
	[]byte("MT"),
	[]byte("ZZ;"),
	[]byte("MT001"),
	[]byte("ID;;"),
	[]byte("MT0011A;B;"),
}

// Run holds d to every conformance property this package can state through
// core/cat's exported API, and fails t with a self-describing message for
// each violation.
//
// Every helper here calls t.Helper(), so a failure is reported at the
// caller's Run(t, d) line rather than inside this package: the message
// carries the detail, and the line number that matters to a model package's
// author is their own.
//
// It is an ERROR to call Run with an unconfigured dialect — see the package
// doc comment for why that is a fatal misuse rather than a silent switch to
// RunZeroValue.
func Run(t *testing.T, d cat.Dialect) {
	t.Helper()

	if !d.Configured() {
		t.Fatalf("dialecttest.Run was given an UNCONFIGURED dialect (Configured() == false, MTForm() == %v). "+
			"This is a misuse, not a conformance failure: an uninitialised package-level var reaches here looking "+
			"exactly like a radio, and reporting PASS for it is the silent green this suite exists to prevent. "+
			"If you meant to check the zero value's refusals, call dialecttest.RunZeroValue(t).", d.MTForm())
	}

	r := &conformanceRun{
		t:        t,
		d:        d,
		frames:   map[string]int{},
		refusals: map[string]int{},
	}

	r.checkAnswerBounds()
	r.checkModeSpace()
	r.checkFixedFrames()
	r.checkSlotFrames()
	r.checkMemoryWrites()
	r.checkMemoryP5()
	r.checkEXReads()
	r.checkFormSeam()
	r.checkGateRefusesTheUnacceptable()
	r.checkNonVacuity()
}

// conformanceRun is one Run's state: the dialect under test, the frame and
// refusal counters that make the properties non-vacuous, and the few facts
// later checks need from earlier ones.
type conformanceRun struct {
	t *testing.T
	d cat.Dialect

	// frames counts built-and-checked frames per builder; refusals counts
	// refusals a check needed to SEE, per kind. A silent skip and an
	// enforced rule are indistinguishable without the second map.
	frames   map[string]int
	refusals map[string]int
	total    int

	// mtMin/mtMax are MTAnswerBounds' answer, captured once and re-used to
	// judge the length of every MT Set frame the dialect builds.
	mtMin, mtMax int
	mtBounds     bool

	// slots is this dialect's own slot space, discovered through ParseSlot,
	// PMSSlot and EMGSlot; modes are the mode bytes it accepts, and
	// emittable excludes ModeUnset, which parsers must accept and builders
	// must never write.
	slots     []cat.Slot
	modes     []cat.Mode
	emittable []cat.Mode

	// firstOwnFormSet and firstClearedSet are own-form MT Set frames kept
	// for the checks that need a genuine one: the wrong-form parser refusal
	// and the over-long-frame refusal.
	firstOwnFormSet []byte
	firstClearedSet []byte

	// exactTagRoundTrips counts NON-EMPTY tags that survived build -> parse
	// byte for byte; trimmedTagRoundTrips counts the short form's legitimate
	// trailing-pad trims.
	exactTagRoundTrips   int
	trimmedTagRoundTrips int
}

// checkFrame is the property the whole suite is built around: this frame is
// well-formed on the wire, and this dialect's own gate admits it.
//
// A builder and a gate that disagree are worse than either being wrong
// alone: it means the program cannot send a command it believes is valid,
// or the gate is not checking what the builder emits.
func (r *conformanceRun) checkFrame(what string, frame []byte) {
	r.t.Helper()

	r.frames[what]++
	r.total++

	if len(frame) == 0 {
		r.t.Errorf("%s: %s produced an empty frame", r.name(), what)
		return
	}
	if frame[len(frame)-1] != ';' {
		r.t.Errorf("%s: %s frame %q does not end with ';'", r.name(), what, frame)
	}
	if n := strings.Count(string(frame), ";"); n != 1 {
		r.t.Errorf("%s: %s frame %q contains %d semicolons, want exactly 1 — a second one splits this into two commands on the wire", r.name(), what, frame, n)
	}
	for i, b := range frame[:len(frame)-1] {
		if !validInteriorByte(b) {
			r.t.Errorf("%s: %s frame %q has byte %#02x at offset %d, outside the permitted interior domain (printable ASCII 0x20-0x7E, excluding ';')", r.name(), what, frame, b, i)
		}
	}
	if !r.d.AllowedCommand(frame) {
		r.t.Errorf("%s: its own gate refused %s frame %q — a builder and a gate that disagree mean this dialect cannot send a command it believes is valid", r.name(), what, frame)
	}
}

// validInteriorByte restates core/cat's own wire-byte domain: printable
// ASCII, no control bytes, and never ';' — which terminates a frame and is
// therefore legal only as the last byte.
//
// It is restated rather than imported because core/cat's predicate is
// unexported, which is the whole condition this package works under. The
// domain is a documented protocol fact (reference: "printable ASCII
// 0x20-0x7E excluding ';' (0x3B)"), not a receiver-varying one, so restating
// it here cannot drift into describing one radio.
func validInteriorByte(b byte) bool {
	return b >= 0x20 && b <= 0x7E && b != ';'
}

// name identifies the dialect under test in every message. CATID is the only
// exported name a dialect has, and it is the one a caller will recognise.
func (r *conformanceRun) name() string {
	return "dialect " + r.d.CATID()
}

// checkAnswerBounds pins MTAnswerBounds against the form it must agree with:
// a range for the short form, EQUAL bounds for the combined form's exact
// length, and — the case a caller must never be able to misread as data — an
// error rather than a plausible (0, 0) for a dialect with no form.
//
// The "equal bounds iff combined" biconditional is checkable in both
// directions from outside, because V9 caps TagMaxBytes at 1 or more: the
// short window's top is its 7-byte floor plus at least one tag byte, so a
// short-form dialect can never report equal bounds.
func (r *conformanceRun) checkAnswerBounds() {
	r.t.Helper()

	min, max, err := r.d.MTAnswerBounds()
	form := r.d.MTForm()

	switch form {
	case cat.MTFormShort, cat.MTFormCombined:
		if err != nil {
			r.t.Errorf("%s: MTAnswerBounds() on a configured %v dialect returned an error: %v", r.name(), form, err)
			return
		}
	default:
		r.t.Errorf("%s: MTForm() = %v — a configured dialect declares a form (NewDialect refuses a config omitting one), so this dialect was built by some route that bypassed validation", r.name(), form)
		if err == nil {
			r.t.Errorf("%s: MTAnswerBounds() returned (%d, %d) and no error for a formless dialect — a caller reading plausible zeros would derive a frame geometry that admits no answer at all, and be told nothing about why", r.name(), min, max)
		}
		return
	}

	if min <= 0 || max <= 0 {
		r.t.Errorf("%s: MTAnswerBounds() = (%d, %d) — an MT answer has a positive length in both forms", r.name(), min, max)
	}
	if max < min {
		r.t.Errorf("%s: MTAnswerBounds() = (%d, %d), an inverted window", r.name(), min, max)
	}
	switch form {
	case cat.MTFormShort:
		if min == max {
			r.t.Errorf("%s: MTAnswerBounds() = (%d, %d) under MTFormShort — the short form's answer is variable-length (a floor plus a tag of 0..TagMaxBytes bytes, TagMaxBytes >= 1), so equal bounds are the COMBINED form's signature and this dialect is reporting the wrong geometry", r.name(), min, max)
		}
	case cat.MTFormCombined:
		if min != max {
			r.t.Errorf("%s: MTAnswerBounds() = (%d, %d) under MTFormCombined — the combined record's length is EXACT, so its bounds must be equal", r.name(), min, max)
		}
	}

	r.mtMin, r.mtMax, r.mtBounds = min, max, true
}

// checkModeSpace walks the whole 8-bit mode wire space through this
// dialect's own ParseMode, because there is no exported enumeration of a
// dialect's modes — deliberately: the mode table is data, and a family with
// a mode this package had never heard of would be invisible to any fixed
// list.
//
// It also holds ParseMode and ValidMode to each other. They are two exported
// answers to the same question, and a dialect whose gate consults one while
// its builders consult the other would emit a mode byte its own gate refuses.
func (r *conformanceRun) checkModeSpace() {
	r.t.Helper()

	for c := 0; c < 256; c++ {
		b := byte(c)
		m, err := r.d.ParseMode(b)
		if err != nil {
			if r.d.ValidMode(cat.Mode(b)) {
				r.t.Errorf("%s: ValidMode(%#02x) is true but ParseMode(%#02x) refused it (%v) — the two must agree, or the gate and the builders are working from different mode tables", r.name(), b, b, err)
			}
			continue
		}
		if !r.d.ValidMode(m) {
			r.t.Errorf("%s: ParseMode(%#02x) accepted the byte but ValidMode(%v) is false", r.name(), b, m)
		}
		if m.Wire() != b {
			r.t.Errorf("%s: ParseMode(%#02x) returned a mode whose Wire() is %#02x — a Mode's underlying byte IS its wire byte", r.name(), b, m.Wire())
		}
		if r.d.ModeName(m) == "" {
			r.t.Errorf("%s: ModeName(%v) is empty for a mode this dialect accepts", r.name(), m)
		}
		r.modes = append(r.modes, m)
		if m != cat.ModeUnset {
			r.emittable = append(r.emittable, m)
		}
	}

	if len(r.modes) == 0 {
		r.t.Errorf("%s: accepts no mode byte at all — no memory record can be built for it and most of this suite would run vacuously", r.name())
	}
	if len(r.emittable) == 0 {
		r.t.Errorf("%s: declares no EMITTABLE mode (ModeUnset is the documented \"-\" placeholder parsers accept and builders must never write) — no Set frame carrying a mode can be built for it", r.name())
	}
}

// checkFixedFrames covers the builders that take no dialect data at all.
// They still go through the same frame property: their output must be clean
// AND admitted by this dialect's gate, and the gate is the half that is
// dialect-dependent even when the builder is not.
func (r *conformanceRun) checkFixedFrames() {
	r.t.Helper()

	r.checkFrame("ID read", r.d.BuildIDRead().Bytes())
	r.checkFrame("AI set (off)", r.d.BuildAISet(false).Bytes())
	r.checkFrame("AI set (on)", r.d.BuildAISet(true).Bytes())
	r.checkFrame("MC read", r.d.BuildMCRead().Bytes())

	// The identity this dialect declares must itself be a legal ID answer:
	// a CATID that its own parser cannot read back is a dialect that can
	// never identify its own radio.
	answer := []byte("ID" + r.d.CATID() + ";")
	got, err := r.d.ParseIDAnswer(answer)
	if err != nil {
		r.t.Errorf("%s: ParseIDAnswer(%q) — its own CATID does not form a parseable ID answer: %v", r.name(), answer, err)
	} else if got != r.d.CATID() {
		r.t.Errorf("%s: ParseIDAnswer(%q) = %q, want its own CATID %q", r.name(), answer, got, r.d.CATID())
	}
}

// threeDigits renders n as the three-digit wire form the slot-bearing
// commands carry, exactly as core/cat's own gate walk does. The whole space
// is swept because a dialect's slot classification is data: nothing outside
// it can enumerate the slots it has.
func threeDigits(n int) string {
	return string([]byte{byte('0' + n/100), byte('0' + (n/10)%10), byte('0' + n%10)})
}

// checkSlotFrames discovers this dialect's slot space and drives every
// slot-bearing builder over it.
//
// PMS slots are reached through PMSSlot rather than the numeric sweep
// because their wire form is "P1L"/"P9U" — the three-digit sweep cannot
// produce them, and a suite that only swept digits would leave a whole slot
// class untested on any family that has one.
func (r *conformanceRun) checkSlotFrames() {
	r.t.Helper()

	for n := 0; n <= 999; n++ {
		if s, err := r.d.ParseSlot(threeDigits(n)); err == nil {
			r.slots = append(r.slots, s)
		}
	}
	// PMSPairs is capped at 9 by validation (the wire form is one digit
	// between 'P' and 'L'/'U'), so this reaches every pair any dialect can
	// declare.
	for pair := 1; pair <= 9; pair++ {
		for _, upper := range []bool{false, true} {
			if s, err := r.d.PMSSlot(pair, upper); err == nil {
				r.slots = append(r.slots, s)
			}
		}
	}
	if s := r.d.EMGSlot(); s.Wire() != "" {
		r.slots = append(r.slots, s)
	}

	if len(r.slots) == 0 {
		r.t.Errorf("%s: classifies no slot at all — every slot-bearing builder below would be skipped and this suite would pass on a dialect that can address no memory", r.name())
		return
	}

	for _, s := range r.slots {
		if cmd, err := r.d.BuildMRRead(s); err == nil {
			r.checkFrame("MR read", cmd.Bytes())
		}
		if cmd, err := r.d.BuildMTRead(s); err == nil {
			r.checkFrame("MT read", cmd.Bytes())
		}
		if cmd, err := r.d.BuildMCSet(s); err == nil {
			r.checkFrame("MC set", cmd.Bytes())
		}
		switch r.d.MTForm() {
		case cat.MTFormShort:
			r.checkShortMTSets(s)
		case cat.MTFormCombined:
			r.checkCombinedMTSets(s)
		}
	}

	// An MR answer cannot be round-tripped from a built frame: BuildMRRead
	// emits a READ request, and only a radio can produce the 28-byte answer
	// that carries the data back. That half is covered in-package against
	// captured frames, and is out of this suite's reach by design.

	r.checkOverlongMTFrameIsRefused()
}

// recordFor is the memory record Run offers a dialect for slot s and mode m,
// carrying the P7 kind the caller names: the combined MT Set's own schema
// value for an MT Set, and this dialect's declared MWWriteKind for an MW Set.
//
// TxClar IS FALSE HERE ON PURPOSE, and it is the ONE value every dialect can
// carry whatever its cat.MemoryP5Policy says. The true case is a per-policy
// truth table rather than a record every walk can use, so it lives in
// checkMemoryP5 below instead of being threaded through every caller.
func recordFor(s cat.Slot, m cat.Mode, kind byte, clarHz int16, ctcss cat.CTCSSState, shift cat.Shift) cat.MemoryData {
	return cat.MemoryData{
		Slot:   s,
		FreqHz: conformanceFreqHz,
		ClarHz: clarHz,
		RxClar: true,
		TxClar: false,
		Mode:   m,
		Kind:   kind,
		CTCSS:  ctcss,
		Shift:  shift,
	}
}

// checkMemoryP5 is byte 21 of the shared memory field block, held to the
// policy this dialect declares — in BOTH directions and through BOTH the
// builders and the gate.
//
// THE TRUTH TABLE, and it is the whole of the check:
//
//	                     | P5TxClar                | P5Fixed
//	---------------------|-------------------------|------------------------
//	TxClar false, build  | builds, gate admits     | builds, gate admits
//	TxClar TRUE,  build  | builds, gate admits     | REFUSED by the builder
//	TxClar TRUE,  parse  | decodes back as true    | REFUSED by the parser
//
// Both halves matter. Without the P5TxClar column a dialect that refused
// every TxClar-true record would pass; without the P5Fixed column one that
// silently encoded the flag as '0' would. The wire byte is asserted too, not
// just the round trip, because a codec that wrote the flag into some OTHER
// position would round-trip perfectly and put a wrong byte on the wire.
func (r *conformanceRun) checkMemoryP5() {
	r.t.Helper()

	if len(r.emittable) == 0 || len(r.slots) == 0 {
		return
	}
	ctcss, shift, ok := r.mustParseStates()
	if !ok {
		return
	}
	writable, found := r.firstWritableSlot(r.emittable[0], ctcss, shift)
	if !found {
		return // checkMemoryWrites has already reported this
	}

	m := recordFor(writable, r.emittable[0], r.d.MWWriteKind(), 0, ctcss, shift)
	m.TxClar = true

	policy := r.d.MemoryP5()
	cmd, err := r.d.BuildMWSet(m)

	switch policy {
	case cat.P5Fixed:
		if err == nil {
			r.t.Errorf("%s: BuildMWSet ACCEPTED a record with TxClar true under %v, emitting %q — this dialect's manual prints P5 \"(Fixed)\", so the flag must be refused rather than silently encoded as '0'", r.name(), policy, cmd.Bytes())
			break
		}
		if !cmd.IsZero() {
			r.t.Errorf("%s: BuildMWSet returned a non-zero Command alongside its P5 refusal; every fallible builder returns the zero Command", r.name())
		}
		r.refusals["TxClar true under P5Fixed"]++

		// AND THE PARSER, on a frame whose byte 21 is '1'. It is spliced
		// into a frame this dialect really did build, so nothing but that
		// one byte can be the reason for the refusal.
		clean, cleanErr := r.d.BuildMWSet(recordFor(writable, r.emittable[0], r.d.MWWriteKind(), 0, ctcss, shift))
		if cleanErr != nil {
			r.t.Errorf("%s: BuildMWSet with TxClar false was refused under %v (%v) — P5Fixed refuses the FLAG, not the record", r.name(), policy, cleanErr)
			return
		}
		r.checkFrame("MW set (P5 fixed)", clean.Bytes())
		forged := append([]byte(nil), clean.Bytes()...)
		if got := forged[memoryP5WireOffset]; got != '0' {
			r.t.Errorf("%s: its own MW Set carries %q at position 21 under %v, want '0' — the printed-fixed byte is not being written where the frame puts it", r.name(), got, policy)
		}
		forged[memoryP5WireOffset] = '1'
		if _, err := r.d.ParseMRAnswer(mrShaped(forged)); err == nil {
			r.t.Errorf("%s: its own parser ACCEPTED %q, whose P5 is '1' under %v — a byte this radio's manual prints \"(Fixed)\" must not be decoded into a flag", r.name(), mrShaped(forged), policy)
		} else {
			r.refusals["P5 '1' at the parser under P5Fixed"]++
		}
		if r.d.AllowedCommand(forged) {
			r.t.Errorf("%s: its gate ADMITTED %q, whose P5 is '1' under %v", r.name(), forged, policy)
		} else {
			r.refusals["P5 '1' at the gate under P5Fixed"]++
		}

	default:
		if err != nil {
			r.t.Errorf("%s: BuildMWSet REFUSED a record with TxClar true under %v (%v) — this dialect's manual prints P5 as the TX clarifier flag, so both its values must be writable", r.name(), policy, err)
			return
		}
		frame := cmd.Bytes()
		r.checkFrame("MW set (TxClar true)", frame)
		if got := frame[memoryP5WireOffset]; got != '1' {
			r.t.Errorf("%s: BuildMWSet with TxClar true emitted %q, whose position 21 is %q rather than '1' — the flag is not reaching the byte the frame puts it in", r.name(), frame, got)
		}
		back, err := r.d.ParseMRAnswer(mrShaped(frame))
		if err != nil {
			r.t.Errorf("%s: ParseMRAnswer(%q) = %v — a frame its own builder produced, with the command prefix swapped, must parse", r.name(), mrShaped(frame), err)
			return
		}
		if !back.TxClar {
			r.t.Errorf("%s: a record built with TxClar true came back with TxClar false under %v — the flag is lost in one direction or the other", r.name(), policy)
		}
	}

	// The COMBINED form shares this byte with MW, and validateCombinedMTFields
	// applies the same policy independently of validateMWFields — so a suite
	// that drove BuildMWSet alone would leave that second validator's P5
	// refusal entirely unexercised from outside core/cat. See LOW-6.
	if r.d.MTForm() == cat.MTFormCombined {
		r.checkCombinedMemoryP5(ctcss, shift)
	}
}

// checkCombinedMemoryP5 is checkMemoryP5's combined-form arm: the same P5
// truth table, driven through buildCombined (BuildMTSetCombined under
// P11Fixed, BuildMTSetCombinedDisplay under P11TagDisplay) rather than
// BuildMWSet, so validateCombinedMTFields' own P5 refusal is exercised from
// outside core/cat too, with its own non-vacuity counters
// (checkNonVacuity's MTFormCombined arm).
func (r *conformanceRun) checkCombinedMemoryP5(ctcss cat.CTCSSState, shift cat.Shift) {
	r.t.Helper()

	var writable cat.Slot
	found := false
	for _, s := range r.slots {
		m := recordFor(s, r.emittable[0], cat.CombinedMTSetKind, 0, ctcss, shift)
		if _, err := r.buildCombined(m, "", false); err == nil {
			writable, found = s, true
			break
		}
	}
	if !found {
		return // checkCombinedMTSets has already reported this
	}

	m := recordFor(writable, r.emittable[0], cat.CombinedMTSetKind, 0, ctcss, shift)
	m.TxClar = true

	policy := r.d.MemoryP5()
	cmd, err := r.buildCombined(m, "", false)

	switch policy {
	case cat.P5Fixed:
		if err == nil {
			r.t.Errorf("%s: BuildMTSetCombined ACCEPTED a record with TxClar true under %v, emitting %q — this dialect's manual prints P5 \"(Fixed)\", so the flag must be refused rather than silently encoded as '0'", r.name(), policy, cmd.Bytes())
			return
		}
		if !cmd.IsZero() {
			r.t.Errorf("%s: BuildMTSetCombined returned a non-zero Command alongside its P5 refusal; every fallible builder returns the zero Command", r.name())
		}
		r.refusals["combined MT set TxClar true under P5Fixed"]++

	default:
		if err != nil {
			r.t.Errorf("%s: BuildMTSetCombined REFUSED a record with TxClar true under %v (%v) — this dialect's manual prints P5 as the TX clarifier flag, so both its values must be writable", r.name(), policy, err)
			return
		}
		frame := cmd.Bytes()
		r.checkFrame("MT set combined (TxClar true)", frame)
		if got := frame[memoryP5WireOffset]; got != '1' {
			r.t.Errorf("%s: combined BuildMTSetCombined with TxClar true emitted %q, whose position 21 is %q rather than '1' — the flag is not reaching the byte the frame puts it in", r.name(), frame, got)
		}
		gotM, _, _, err := r.parseCombined(frame)
		if err != nil {
			r.t.Errorf("%s: parseCombined(%q) = %v — a frame its own builder produced must parse", r.name(), frame, err)
			return
		}
		if !gotM.TxClar {
			r.t.Errorf("%s: a combined record built with TxClar true came back with TxClar false under %v — the flag is lost in one direction or the other", r.name(), policy)
		}
	}
}

// memoryP5WireOffset is position 21 of the shared memory field block,
// 0-indexed — the byte cat.MemoryP5Policy governs.
//
// It is restated here rather than imported because core/cat's offsets are
// unexported, which is the condition this package works under. It is a
// documented protocol fact (the manuals' own position tables number P5 at
// 21) and not a receiver-varying one, so restating it cannot drift into
// describing one radio. What it must NOT become is a second opinion: the
// checks above assert that this dialect's own builder puts the flag HERE,
// which is what would fail if core/cat ever moved the field.
const memoryP5WireOffset = 20

// mrShaped returns frame with its two-byte command prefix rewritten to "MR",
// so an MW Set this package just built can be offered to the MR answer
// parser.
//
// The two frames are byte-for-byte identical in every documented position
// except that prefix — core/cat's memdata.go says so, and this package's
// only route to the memory-record DECODER is ParseMRAnswer, since MW has no
// Answer form and no exported parser. Without this the read direction of the
// P5 policy would be unreachable from outside core/cat altogether.
func mrShaped(frame []byte) []byte {
	out := append([]byte(nil), frame...)
	if len(out) >= 2 {
		out[0], out[1] = 'M', 'R'
	}
	return out
}

// mustParseStates returns the CTCSS and shift states a P8/P10 wire byte of
// '0' means — off and simplex — reporting a failure rather than a panic if
// this dialect's own package cannot parse the documented bytes.
func (r *conformanceRun) mustParseStates() (cat.CTCSSState, cat.Shift, bool) {
	r.t.Helper()

	ctcss, err := cat.ParseCTCSSState('0')
	if err != nil {
		r.t.Errorf("cat.ParseCTCSSState('0') = %v, want the documented \"off\" state", err)
		return 0, 0, false
	}
	shift, err := cat.ParseShift('0')
	if err != nil {
		r.t.Errorf("cat.ParseShift('0') = %v, want the documented \"simplex\" state", err)
		return 0, 0, false
	}
	return ctcss, shift, true
}

// checkShortMTSets drives the short form's Set builder over slot s, in both
// display states and over every candidate tag, and round-trips each frame
// through this dialect's own answer parser.
//
// THE EMPTY TAG IS THE EXACT CASE. It is written as a full field of this
// dialect's own clear byte and must come back as "" on every dialect — the
// two halves of the tag-normalisation fix meeting. A non-empty tag may come
// back TRIMMED, and legitimately so: the short form declares a pad byte
// because the radio pads a short tag on read, so a tag ending in that byte
// does not survive the round trip. That is the form's own semantics, not a
// defect, which is why it is tolerated here — but only as trailing-byte
// trimming, and only if some other candidate survives intact.
func (r *conformanceRun) checkShortMTSets(s cat.Slot) {
	r.t.Helper()

	for _, display := range []bool{false, true} {
		if cmd, err := r.d.BuildMTSet(s, display, ""); err == nil {
			frame := cmd.Bytes()
			r.checkFrame("MT set short (cleared)", frame)
			r.checkMTFrameLength("MT set short (cleared)", frame, true)
			r.checkShortRoundTrip(frame, s, display, "", true)
			r.keepOwnFormFrames(frame, true)
		}
		for _, tag := range conformanceTags {
			cmd, err := r.d.BuildMTSet(s, display, tag)
			if err != nil {
				// A tag wider than this dialect's field, or a slot its MT
				// write policy refuses. Neither is this check's business;
				// the floors below catch a builder refusing everything.
				continue
			}
			frame := cmd.Bytes()
			r.checkFrame("MT set short (tagged)", frame)
			r.checkMTFrameLength("MT set short (tagged)", frame, false)
			r.checkShortRoundTrip(frame, s, display, tag, false)
			r.keepOwnFormFrames(frame, false)
		}
	}
}

// checkShortRoundTrip parses a frame the short-form builder just produced
// and holds the result to the input.
func (r *conformanceRun) checkShortRoundTrip(frame []byte, want cat.Slot, wantDisplay bool, wantTag string, tagMustBeExact bool) {
	r.t.Helper()

	slot, display, tag, err := r.d.ParseMTAnswer(frame)
	if err != nil {
		r.t.Errorf("%s: ParseMTAnswer(%q) = %v — a frame its own builder produced must parse", r.name(), frame, err)
		return
	}
	if slot.Wire() != want.Wire() {
		r.t.Errorf("%s: ParseMTAnswer(%q) returned slot %q, want %q", r.name(), frame, slot.Wire(), want.Wire())
	}
	if display != wantDisplay {
		r.t.Errorf("%s: ParseMTAnswer(%q) returned display %v, want %v", r.name(), frame, display, wantDisplay)
	}
	if tag == wantTag {
		if wantTag != "" {
			r.exactTagRoundTrips++
		}
		return
	}
	if tagMustBeExact {
		r.t.Errorf("%s: ParseMTAnswer(%q) returned tag %q, want %q exactly — an empty tag is written as the clear form and must decode back to no tag at all", r.name(), frame, tag, wantTag)
		return
	}
	if !trailingTrimOnly(wantTag, tag) {
		r.t.Errorf("%s: built tag %q came back as %q — the only difference a short-form round trip may introduce is the trimming of trailing pad bytes, so this is data loss the wire encoding does not account for", r.name(), wantTag, tag)
		return
	}
	r.trimmedTagRoundTrips++
}

// trailingTrimOnly reports whether got is offered with a run of one repeated
// trailing byte removed — the only difference a pad-byte trim can produce.
//
// It is deliberately strict about all three properties a TrimRight leaves
// behind: got is a PREFIX of offered (so no byte was substituted or
// reordered), the removed suffix is ONE byte repeated (so a trim of two
// different bytes is still a failure), and what remains does not end in that
// same byte (a greedy trim would have taken it too, so anything else means
// the parser is doing something other than trimming).
func trailingTrimOnly(offered, got string) bool {
	if !strings.HasPrefix(offered, got) {
		return false
	}
	suffix := offered[len(got):]
	if suffix == "" {
		return false
	}
	for i := 0; i < len(suffix); i++ {
		if suffix[i] != suffix[0] {
			return false
		}
	}
	return got == "" || got[len(got)-1] != suffix[0]
}

// checkCombinedMTSets drives the combined form's Set builder over slot s and
// round-trips every frame it produces.
//
// EVERY ACCEPTED TAG MUST ROUND-TRIP EXACTLY here, with none of the short
// form's tolerance — because this form's builder REFUSES a tag ending in the
// fill byte outright rather than letting it be silently trimmed on the way
// back. The two forms differ on this by design, and the difference is
// visible from outside precisely as it should be: one refuses, the other
// trims.
func (r *conformanceRun) checkCombinedMTSets(s cat.Slot) {
	r.t.Helper()

	if len(r.emittable) == 0 {
		return
	}
	ctcss, shift, ok := r.mustParseStates()
	if !ok {
		return
	}
	// cat.CombinedMTSetKind IS the combined Set's P7 schema constant, named
	// directly. In the SET direction the reference documents the byte
	// "(Fixed)", not "VFO"; before M9c-4 core/cat spelt that meaning with an
	// unexported constant and a package outside it had only the byte's
	// read-side name (cat.KindVFO) to write, which said the wrong thing at
	// the right value. The rename closed that. The assertion on the parsed
	// record below is what pins that the byte survived.
	m := recordFor(s, r.emittable[0], cat.CombinedMTSetKind, 0, ctcss, shift)

	for _, tag := range append([]string{""}, conformanceTags...) {
		what := "MT set combined (tagged)"
		if tag == "" {
			// The combined form documents no distinct clear encoding: an
			// empty tag IS the all-fill field.
			what = "MT set combined (cleared)"
		}
		cmd, err := r.buildCombined(m, tag, true)
		if err != nil {
			if tag == "" {
				// Not tolerable: the empty tag is the one every combined
				// dialect must be able to write for a slot it accepts.
				if !r.slotTakesACombinedSet(m) {
					continue
				}
				r.t.Errorf("%s: BuildMTSetCombined(slot %q, \"\") = %v — a slot this dialect otherwise writes must accept the cleared (all-fill) tag", r.name(), s.Wire(), err)
			}
			// A non-empty candidate may be refused for reasons that are the
			// form working correctly: too wide for this field, or ending in
			// this dialect's fill byte.
			continue
		}
		frame := cmd.Bytes()
		r.checkFrame(what, frame)
		r.checkMTFrameLength(what, frame, true)
		r.keepOwnFormFrames(frame, tag == "")

		gotM, gotTag, gotDisplay, err := r.parseCombined(frame)
		if err != nil {
			r.t.Errorf("%s: ParseMTAnswerCombined(%q) = %v — a frame its own builder produced must parse", r.name(), frame, err)
			continue
		}
		if gotM != m {
			r.t.Errorf("%s: %s for slot %q round-tripped to %+v, want %+v", r.name(), what, s.Wire(), gotM, m)
		}
		if gotTag != tag {
			r.t.Errorf("%s: %s for slot %q round-tripped tag %q, want %q — the fill padding is a wire-encoding concern and must not survive the parse", r.name(), what, s.Wire(), gotTag, tag)
			continue
		}
		if !gotDisplay {
			r.t.Errorf("%s: %s for slot %q was built with the TAG flag ON and came back OFF — under %v byte 28 is a live flag, and one that does not survive its own round trip is not being carried", r.name(), what, s.Wire(), r.d.MTP11())
		}
		if tag != "" {
			r.exactTagRoundTrips++
		}
	}

	r.checkCombinedP11Seam(m)
}

// buildCombined calls whichever combined Set builder this dialect's P11
// policy answers to, with display as the TAG flag under P11TagDisplay and
// ignored under P11Fixed.
//
// The two are not interchangeable: a live flag is never defaulted, so the
// display-less builder REFUSES a P11TagDisplay dialect and the
// display-bearing one refuses a P11Fixed one. checkCombinedP11Seam is where
// that pair of refusals is required to be SEEN; this helper is just the
// right call.
func (r *conformanceRun) buildCombined(m cat.MemoryData, tag string, display bool) (cat.Command, error) {
	if r.d.MTP11() == cat.P11TagDisplay {
		return r.d.BuildMTSetCombinedDisplay(m, tag, display)
	}
	return r.d.BuildMTSetCombined(m, tag)
}

// parseCombined is buildCombined's counterpart. Under P11Fixed the flag it
// reports is false, because byte 28 carries no state there.
func (r *conformanceRun) parseCombined(frame []byte) (cat.MemoryData, string, bool, error) {
	if r.d.MTP11() == cat.P11TagDisplay {
		return r.d.ParseMTAnswerCombinedDisplay(frame)
	}
	m, tag, err := r.d.ParseMTAnswerCombined(frame)
	// Under P11Fixed the byte is the printed "(Fixed)" and there is no flag
	// to report; true is returned so the round-trip assertion above states
	// something only where a flag exists, rather than failing every
	// P11Fixed dialect on a value it does not have.
	return m, tag, true, err
}

// checkCombinedP11Seam requires the WRONG P11 pair to refuse, and to be SEEN
// to refuse — all four refusals, both builders and both parsers.
//
// It is checkFormSeam's rule applied one level down. A live flag is never
// defaulted, so a P11TagDisplay dialect must refuse the display-less builder
// rather than quietly writing '0' for a flag the caller never expressed an
// intention about; and a P11Fixed dialect must refuse the display-bearing
// one rather than letting a caller set a flag its radio does not have. A
// call that was skipped and a seam that is enforced look identical from
// outside, which is why every refusal below is counted.
func (r *conformanceRun) checkCombinedP11Seam(m cat.MemoryData) {
	r.t.Helper()

	if r.d.MTForm() != cat.MTFormCombined {
		return
	}
	// A frame this dialect really did build, so the parser refusals below
	// can only be about the API pairing.
	cmd, err := r.buildCombined(m, "", false)
	if err != nil {
		return // the empty-tag assertion above has already reported this
	}
	frame := cmd.Bytes()

	tagDisplay := r.d.MTP11() == cat.P11TagDisplay

	wrongBuild, wrongBuildErr := r.d.BuildMTSetCombined(m, "")
	if tagDisplay {
		r.requireP11Refusal("display-less combined build under P11TagDisplay", wrongBuild, wrongBuildErr)
	} else if wrongBuildErr != nil {
		r.t.Errorf("%s: BuildMTSetCombined was refused (%v) on a %v dialect — it is that policy's OWN builder", r.name(), wrongBuildErr, r.d.MTP11())
	}

	wrongDisplayBuild, wrongDisplayErr := r.d.BuildMTSetCombinedDisplay(m, "", true)
	if tagDisplay {
		if wrongDisplayErr != nil {
			r.t.Errorf("%s: BuildMTSetCombinedDisplay was refused (%v) on a %v dialect — it is that policy's OWN builder", r.name(), wrongDisplayErr, r.d.MTP11())
		}
	} else {
		r.requireP11Refusal("display-bearing combined build under P11Fixed", wrongDisplayBuild, wrongDisplayErr)
	}

	if _, _, err := r.d.ParseMTAnswerCombined(frame); (err == nil) == tagDisplay {
		if tagDisplay {
			r.t.Errorf("%s: ParseMTAnswerCombined ACCEPTED %q on a %v dialect — byte 28 is a live flag there, and a parser that drops it hands back a record the radio did not send", r.name(), frame, r.d.MTP11())
		} else {
			r.t.Errorf("%s: ParseMTAnswerCombined refused %q on a %v dialect — it is that policy's OWN parser", r.name(), frame, r.d.MTP11())
		}
	} else if tagDisplay {
		r.refusals["display-less combined parse under P11TagDisplay"]++
	}

	if _, _, _, err := r.d.ParseMTAnswerCombinedDisplay(frame); (err == nil) != tagDisplay {
		if tagDisplay {
			r.t.Errorf("%s: ParseMTAnswerCombinedDisplay refused %q on a %v dialect — it is that policy's OWN parser", r.name(), frame, r.d.MTP11())
		} else {
			r.t.Errorf("%s: ParseMTAnswerCombinedDisplay ACCEPTED %q on a %v dialect — byte 28 is the printed \"(Fixed)\" there, and reporting it as a flag reads schema as state", r.name(), frame, r.d.MTP11())
		}
	} else if !tagDisplay {
		r.refusals["display-bearing combined parse under P11Fixed"]++
	}

	// THE FLAG MUST REACH THE WIRE, and be seen to. Under P11TagDisplay a
	// TAG-ON frame and a TAG-OFF frame must differ at byte 28 and nowhere
	// else, which is what makes the round trip above evidence of a flag
	// rather than of a constant.
	if !tagDisplay {
		return
	}
	on, errOn := r.buildCombined(m, "", true)
	if errOn != nil {
		r.t.Errorf("%s: BuildMTSetCombinedDisplay with the TAG flag ON = %v", r.name(), errOn)
		return
	}
	off, errOff := r.buildCombined(m, "", false)
	if errOff != nil {
		r.t.Errorf("%s: BuildMTSetCombinedDisplay with the TAG flag OFF = %v", r.name(), errOff)
		return
	}
	r.checkFrame("MT set combined (TAG on)", on.Bytes())
	diffs := 0
	a, b := on.Bytes(), off.Bytes()
	if len(a) != len(b) {
		r.t.Errorf("%s: the TAG-on and TAG-off combined frames differ in LENGTH (%d vs %d) — the flag is one byte", r.name(), len(a), len(b))
		return
	}
	for i := range a {
		if a[i] != b[i] {
			diffs++
		}
	}
	if diffs != 1 {
		r.t.Errorf("%s: the TAG-on and TAG-off combined frames differ in %d bytes (%q vs %q), want exactly 1 — byte 28 is the flag and nothing else may move with it", r.name(), diffs, a, b)
	}
}

// requireP11Refusal holds one wrong-pair call to the refusal contract: an
// error, the zero Command, and a message naming this dialect's own policy —
// a refusal for some other reason would leave the seam unproven while
// looking exactly like proof of it.
func (r *conformanceRun) requireP11Refusal(what string, cmd cat.Command, err error) {
	r.t.Helper()

	if err == nil {
		r.t.Errorf("%s: %s SUCCEEDED, emitting %q — the P11 seam is not being enforced", r.name(), what, cmd.Bytes())
		return
	}
	if !cmd.IsZero() {
		r.t.Errorf("%s: %s returned a non-zero Command alongside its error; every fallible builder returns the zero Command", r.name(), what)
	}
	r.refusals[what]++
}

// slotTakesACombinedSet reports whether this dialect will build a combined
// Set for m's slot at all, asked with a tag that cannot itself be the reason
// for a refusal.
//
// It exists so the empty-tag assertion above stays about the TAG. Most slots
// a dialect classifies are not MT-writable (5xx and EMG are refused by
// project policy, "000" by the reference), and a suite that reported those
// as tag failures would be noise.
func (r *conformanceRun) slotTakesACombinedSet(m cat.MemoryData) bool {
	r.t.Helper()

	for _, tag := range conformanceTags {
		if _, err := r.d.BuildMTSetCombined(m, tag); err == nil {
			return true
		}
	}
	return false
}

// checkMTFrameLength holds every MT Set frame to the geometry the dialect
// reports for itself. atMax is set for the frames that fill the tag field —
// the cleared short form (a full field of clear bytes) and every combined
// record (fixed width) — which must sit exactly at the window's top.
func (r *conformanceRun) checkMTFrameLength(what string, frame []byte, atMax bool) {
	r.t.Helper()

	if !r.mtBounds {
		return
	}
	if len(frame) < r.mtMin || len(frame) > r.mtMax {
		r.t.Errorf("%s: %s frame %q is %d bytes, outside the %d-%d window this dialect reports from MTAnswerBounds() — a frame it builds but its own geometry excludes could not be read back", r.name(), what, frame, len(frame), r.mtMin, r.mtMax)
		return
	}
	if atMax && len(frame) != r.mtMax {
		r.t.Errorf("%s: %s frame %q is %d bytes, want the window's top %d — this form fills the tag field, so its frame is the longest this dialect can produce", r.name(), what, frame, len(frame), r.mtMax)
	}
}

// keepOwnFormFrames retains one own-form Set frame of each kind for the
// checks that need a genuine one.
func (r *conformanceRun) keepOwnFormFrames(frame []byte, cleared bool) {
	if r.firstOwnFormSet == nil {
		r.firstOwnFormSet = append([]byte(nil), frame...)
	}
	if cleared && r.firstClearedSet == nil {
		r.firstClearedSet = append([]byte(nil), frame...)
	}
}

// checkOverlongMTFrameIsRefused splices one more tag byte into a frame that
// already sits at the top of this dialect's answer window, and requires both
// the parser and the outbound gate to refuse the result.
//
// This is the receiver-derived window doing its job, checked from outside:
// before M9c-3 the window was one package constant sized to the FT-710's
// 12-byte tag, so a 6-byte-tag family had frames up to 19 bytes admitted as
// "within the window" with only the tag charset between a forged frame and
// the radio. A dialect that let this frame through would have exactly that
// defect back.
func (r *conformanceRun) checkOverlongMTFrameIsRefused() {
	r.t.Helper()

	if r.firstClearedSet == nil {
		return // the floors below report the absent builder
	}
	base := r.firstClearedSet
	over := make([]byte, 0, len(base)+1)
	over = append(over, base[:len(base)-1]...)
	over = append(over, 'X', ';')

	var err error
	switch r.d.MTForm() {
	case cat.MTFormShort:
		_, _, _, err = r.d.ParseMTAnswer(over)
	case cat.MTFormCombined:
		_, _, err = r.d.ParseMTAnswerCombined(over)
	}
	if err == nil {
		r.t.Errorf("%s: its own answer parser ACCEPTED %q, one byte past the %d-byte top of the window it reports — the answer window is not deriving from this dialect", r.name(), over, r.mtMax)
	} else {
		r.refusals["over-long MT answer"]++
	}
	if r.d.AllowedCommand(over) {
		r.t.Errorf("%s: its own gate ADMITTED %q, one byte past the %d-byte top of the window it reports — the outbound gate is judging MT frames by some other dialect's tag width", r.name(), over, r.mtMax)
	} else {
		r.refusals["over-long MT frame at the gate"]++
	}
}

// checkMemoryWrites drives BuildMWSet — the builder through which a
// dialect's mode table, its clarifier policy and its declared MW write kind
// ALL reach the wire. Every other builder emits a fixed form or a slot.
func (r *conformanceRun) checkMemoryWrites() {
	r.t.Helper()

	if len(r.emittable) == 0 || len(r.slots) == 0 {
		return
	}
	ctcss, shift, ok := r.mustParseStates()
	if !ok {
		return
	}

	// One MW Set per emittable mode: a mode key that could not reach the
	// wire cleanly — the NUL nibble the adversarial-config property was
	// written for — is only visible here.
	for _, m := range r.emittable {
		for _, s := range r.slots {
			cmd, err := r.d.BuildMWSet(recordFor(s, m, r.d.MWWriteKind(), 0, ctcss, shift))
			if err != nil {
				continue
			}
			r.checkFrame("MW set", cmd.Bytes())
			break // one slot per mode; the slot space is already swept above
		}
	}

	writable, found := r.firstWritableSlot(r.emittable[0], ctcss, shift)
	if !found {
		r.t.Errorf("%s: no slot in its own slot space accepts an MW Set — this dialect can read memories and write none of them", r.name())
		return
	}

	// The CTCSS and shift wire spaces, exhaustively, through the exported
	// parsers. Both are byte-alias types, so these are the only exported
	// route to a value a builder will accept.
	for c := 0; c < 256; c++ {
		state, err := cat.ParseCTCSSState(byte(c))
		if err != nil {
			continue
		}
		for s := 0; s < 256; s++ {
			sh, err := cat.ParseShift(byte(s))
			if err != nil {
				continue
			}
			cmd, err := r.d.BuildMWSet(recordFor(writable, r.emittable[0], r.d.MWWriteKind(), 0, state, sh))
			if err != nil {
				r.t.Errorf("%s: BuildMWSet with CTCSS %#02x and shift %#02x — both parsed from their own exported parsers — was refused: %v", r.name(), c, s, err)
				continue
			}
			r.checkFrame("MW set (CTCSS/shift matrix)", cmd.Bytes())
		}
	}

	// The clarifier, at the endpoints of the policy this dialect declares
	// and one step past them. The endpoints must BUILD (V-rules require
	// MaxAbsHz to be a multiple of StepHz, so the range's own top is a legal
	// value) and one step further must be REFUSED before any wire traffic.
	clar := r.d.Clarifier()
	for _, hz := range []int{0, clar.MaxAbsHz, -clar.MaxAbsHz} {
		cmd, err := r.d.BuildMWSet(recordFor(writable, r.emittable[0], r.d.MWWriteKind(), int16(hz), ctcss, shift))
		if err != nil {
			r.t.Errorf("%s: BuildMWSet with a clarifier of %d Hz was refused (%v) — this dialect declares StepHz %d and MaxAbsHz %d, so its own range's endpoint must be writable", r.name(), hz, err, clar.StepHz, clar.MaxAbsHz)
			continue
		}
		r.checkFrame("MW set (clarifier endpoint)", cmd.Bytes())
	}
	// The 4-digit clarifier field bounds every dialect well inside int16, so
	// this addition cannot overflow; the guard states the bound rather than
	// assuming a reader will check the validator.
	if over := clar.MaxAbsHz + clar.StepHz; over <= 32767 {
		cmd, err := r.d.BuildMWSet(recordFor(writable, r.emittable[0], r.d.MWWriteKind(), int16(over), ctcss, shift))
		if err == nil {
			r.t.Errorf("%s: BuildMWSet ACCEPTED a clarifier of %d Hz, one step past its declared MaxAbsHz %d, emitting %q", r.name(), over, clar.MaxAbsHz, cmd.Bytes())
		} else {
			if !cmd.IsZero() {
				r.t.Errorf("%s: BuildMWSet returned a non-zero Command alongside its out-of-range clarifier error; every fallible builder returns the zero Command", r.name())
			}
			r.refusals["clarifier past MaxAbsHz"]++
		}
	}
}

// firstWritableSlot returns the first slot this dialect will accept an MW
// Set for.
func (r *conformanceRun) firstWritableSlot(m cat.Mode, ctcss cat.CTCSSState, shift cat.Shift) (cat.Slot, bool) {
	r.t.Helper()

	for _, s := range r.slots {
		if _, err := r.d.BuildMWSet(recordFor(s, m, r.d.MWWriteKind(), 0, ctcss, shift)); err == nil {
			return s, true
		}
	}
	return cat.Slot{}, false
}

// checkEXReads drives BuildEXRead over this dialect's OWN menu inventory,
// and holds EXAddresses and KnownEXAddress to each other: an address the
// dialect lists but does not recognise would make every EX answer it
// received unattributable.
//
// EVERY LENGTH HERE IS THIS DIALECT'S OWN. The EX address field is four
// digits on some radios and six on others (cat.EXAddressForm), so the read
// frame is 2 + d.EXAddressWidth() + 1 bytes and the address occupies
// exactly the width d.EXWire renders. Written against a constant 9 this
// suite would have declared a four-digit dialect non-conformant while its
// builder, gate and parser all agreed with each other.
//
// The answer round-trip is the other half: the narrowest answer this
// dialect can receive at each address must parse back to that same address.
// A read frame that is well-formed and a parser that cannot read the reply
// is a dialect that can ask a question and not hear the answer.
//
// An empty inventory is legal (DialectConfig documents it: "a radio with no
// menu inventory described yet"), so it is logged rather than failed.
func (r *conformanceRun) checkEXReads() {
	r.t.Helper()

	addrs := r.d.EXAddresses()
	if len(addrs) == 0 {
		r.t.Logf("%s: declares no EX inventory, so the EX half of this suite is empty for it (DialectConfig permits this)", r.name())
		return
	}
	width := r.d.EXAddressWidth()
	if width <= 0 {
		r.t.Errorf("%s: EXAddressWidth() = %d for a dialect with %d inventory addresses — it can render no address field, so none of its EX frames can be built", r.name(), width, len(addrs))
		return
	}
	roundTrips := 0
	for _, a := range addrs {
		if !r.d.KnownEXAddress(a) {
			r.t.Errorf("%s: KnownEXAddress(%v) is false for an address its own EXAddresses() lists", r.name(), a)
		}
		wire := r.d.EXWire(a)
		if len(wire) != width {
			r.t.Errorf("%s: EXWire(%v) is %d bytes but EXAddressWidth() says %d — the width and the render have come apart", r.name(), a, len(wire), width)
			continue
		}
		cmd, err := r.d.BuildEXRead(a)
		if err != nil {
			r.t.Errorf("%s: BuildEXRead(%v) = %v — an address from its own inventory must build", r.name(), a, err)
			continue
		}
		r.checkFrame("EX read", cmd.Bytes())

		frame := cmd.Bytes()
		if len(frame) != 2+width+1 {
			r.t.Errorf("%s: BuildEXRead(%v) is %d bytes, want %d (\"EX\" + a %d-byte address + \";\")", r.name(), a, len(frame), 2+width+1, width)
			continue
		}
		if got := string(frame[2 : 2+width]); got != wire {
			r.t.Errorf("%s: BuildEXRead(%v) carries address field %q, want %q", r.name(), a, got, wire)
		}

		// The narrowest answer this dialect can receive: one P4 byte.
		answer := []byte("EX" + wire + "0;")
		gotAddr, raw, err := r.d.ParseEXAnswer(answer)
		if err != nil {
			r.t.Errorf("%s: ParseEXAnswer(%q) = %v — this is the narrowest answer to a read this same dialect just built", r.name(), answer, err)
			continue
		}
		if gotAddr != a {
			r.t.Errorf("%s: ParseEXAnswer(%q) returned address %v, want %v", r.name(), answer, gotAddr, a)
			continue
		}
		if raw != "0" {
			r.t.Errorf("%s: ParseEXAnswer(%q) returned P4 %q, want %q verbatim", r.name(), answer, raw, "0")
			continue
		}
		roundTrips++
	}
	// Non-vacuity, in this suite's usual sense: a loop that round-tripped
	// nothing passes in silence.
	if roundTrips == 0 {
		r.t.Errorf("%s: no EX answer round-tripped, though this dialect lists %d addresses — the property above passed vacuously", r.name(), len(addrs))
	}
}

// checkFormSeam requires the WRONG form's API to refuse, and to be SEEN to
// refuse: a call that was skipped and a seam that is enforced look identical
// from the outside.
//
// The refusal must NAME this dialect's form. An error for some other reason
// — an invalid slot, an empty record — would leave the seam unproven while
// looking exactly like proof of it, which is why the wrong-form BUILD is
// offered a deliberately empty record: only a form check running FIRST can
// produce a message naming the form.
func (r *conformanceRun) checkFormSeam() {
	r.t.Helper()

	form := r.d.MTForm()
	var (
		cmd cat.Command
		err error
	)
	switch form {
	case cat.MTFormShort:
		cmd, err = r.d.BuildMTSetCombined(cat.MemoryData{}, "AB")
	case cat.MTFormCombined:
		if len(r.slots) == 0 {
			return
		}
		cmd, err = r.d.BuildMTSet(r.slots[0], false, "AB")
	default:
		return
	}
	if err == nil {
		r.t.Errorf("%s: the WRONG-form MT Set builder succeeded on a %v dialect, emitting %q — the two forms share a prefix and a terminator, so a frame built in the wrong shape looks plausible all the way to the radio", r.name(), form, cmd.Bytes())
	} else {
		if !strings.Contains(err.Error(), form.String()) {
			r.t.Errorf("%s: the wrong-form MT Set builder was refused by %q, which does not name this dialect's form (%v) — a refusal for some other reason proves nothing about the seam", r.name(), err, form)
		}
		if !cmd.IsZero() {
			r.t.Errorf("%s: the wrong-form MT Set builder returned a non-zero Command alongside its error; every fallible builder returns the zero Command", r.name())
		}
		r.refusals["wrong-form MT Set build"]++
	}

	// And the wrong-form PARSER, against a frame this dialect really did
	// build. This is the direction that would otherwise decode a frame at
	// the wrong offsets and hand back data rather than an error.
	if r.firstOwnFormSet == nil {
		return
	}
	switch form {
	case cat.MTFormShort:
		_, _, err = r.d.ParseMTAnswerCombined(r.firstOwnFormSet)
	case cat.MTFormCombined:
		_, _, _, err = r.d.ParseMTAnswer(r.firstOwnFormSet)
	}
	if err == nil {
		r.t.Errorf("%s: the WRONG-form MT answer parser accepted %q on a %v dialect — it would return data read from another form's offsets", r.name(), r.firstOwnFormSet, form)
		return
	}
	if !strings.Contains(err.Error(), form.String()) {
		r.t.Errorf("%s: the wrong-form MT answer parser was refused by %q, which does not name this dialect's form (%v)", r.name(), err, form)
	}
	r.refusals["wrong-form MT answer parse"]++
}

// checkGateRefusesTheUnacceptable is what stops "its own gate admits every
// frame it builds" from being satisfied by a gate that admits everything.
func (r *conformanceRun) checkGateRefusesTheUnacceptable() {
	r.t.Helper()

	for _, frame := range universallyRefusedFrames {
		if r.d.AllowedCommand(frame) {
			r.t.Errorf("%s: its gate ADMITTED %q — no dialect may admit an unknown command, an unterminated frame, or one carrying a second command after an interior ';'", r.name(), frame)
			continue
		}
		r.refusals["malformed frame at the gate"]++
	}
}

// checkNonVacuity is the half of this suite that fails when nothing
// happened. Every property above is a for-loop over data the dialect
// supplies, and a dialect supplying none would satisfy all of them in
// silence — which is exactly how a fixture with a mistyped slot space, or a
// builder dropped from the walk, goes unnoticed.
func (r *conformanceRun) checkNonVacuity() {
	r.t.Helper()

	required := []string{
		"ID read", "AI set (off)", "AI set (on)", "MC read",
		"MR read", "MT read", "MC set",
		"MW set", "MW set (CTCSS/shift matrix)", "MW set (clarifier endpoint)",
	}
	switch r.d.MTForm() {
	case cat.MTFormShort:
		required = append(required, "MT set short (tagged)", "MT set short (cleared)")
	case cat.MTFormCombined:
		required = append(required, "MT set combined (tagged)", "MT set combined (cleared)")
	}
	// The P5 arm this dialect's own policy selects. Both arms are counted,
	// because "the check ran" and "the check ran for THIS policy" are
	// different claims: a suite that only required the P5TxClar frame would
	// go green on a P5Fixed dialect whose whole arm had been skipped.
	requiredRefusals := []string{
		"wrong-form MT Set build",
		"wrong-form MT answer parse",
		"over-long MT answer",
		"over-long MT frame at the gate",
		"clarifier past MaxAbsHz",
		"malformed frame at the gate",
	}
	switch r.d.MTP11() {
	case cat.P11TagDisplay:
		required = append(required, "MT set combined (TAG on)")
		requiredRefusals = append(requiredRefusals,
			"display-less combined build under P11TagDisplay",
			"display-less combined parse under P11TagDisplay")
	case cat.P11Fixed:
		requiredRefusals = append(requiredRefusals,
			"display-bearing combined build under P11Fixed",
			"display-bearing combined parse under P11Fixed")
	}
	switch r.d.MemoryP5() {
	case cat.P5Fixed:
		required = append(required, "MW set (P5 fixed)")
		requiredRefusals = append(requiredRefusals,
			"TxClar true under P5Fixed",
			"P5 '1' at the parser under P5Fixed",
			"P5 '1' at the gate under P5Fixed")
		if r.d.MTForm() == cat.MTFormCombined {
			requiredRefusals = append(requiredRefusals, "combined MT set TxClar true under P5Fixed")
		}
	default:
		required = append(required, "MW set (TxClar true)")
		if r.d.MTForm() == cat.MTFormCombined {
			required = append(required, "MT set combined (TxClar true)")
		}
	}
	for _, what := range required {
		if r.frames[what] == 0 {
			r.t.Errorf("%s: builder %q contributed no frames — either this dialect refuses it for every input this suite can offer, or the builder was dropped from the walk, and both are defects this property must not pass over", r.name(), what)
		}
	}

	for _, what := range requiredRefusals {
		if r.refusals[what] == 0 {
			r.t.Errorf("%s: no %q refusal was observed — the check either never ran or never had anything to refuse, and a rule that was never exercised is not evidence that it is enforced", r.name(), what)
		}
	}

	if r.exactTagRoundTrips == 0 {
		r.t.Errorf("%s: not one non-empty tag survived build -> parse intact (%d came back trimmed) — this suite offers tags with four different final bytes precisely so that a dialect's fill and pad bytes cannot spoil them all, so a dialect losing every one of them has an MT tag codec that does not round-trip", r.name(), r.trimmedTagRoundTrips)
	}
	if r.total < minConformanceFrames {
		r.t.Errorf("%s: only %d frames were built and checked in total; a dialect with a slot space, a mode table and a menu produces hundreds — this walk is not reaching the builders", r.name(), r.total)
	}

	r.t.Logf("%s (%v): %d frames checked across %d slots and %d modes; per builder: %v; refusals seen: %v; tag round trips: %d exact, %d trimmed",
		r.name(), r.d.MTForm(), r.total, len(r.slots), len(r.modes), r.frames, r.refusals, r.exactTagRoundTrips, r.trimmedTagRoundTrips)
}

// RunZeroValue holds the ZERO cat.Dialect to the one property it has: it
// refuses everything it is offered.
//
// A separate exported entry point rather than something Run detects — the
// package doc comment gives the reason at length. The short version is that
// an uninitialised dialect reaching a conformance suite must be a loud
// failure, not a different suite quietly passing.
//
// THREE BUILDERS DO STILL EMIT, and they are asserted here rather than left
// to look like gaps: BuildIDRead, BuildAISet and BuildMCRead take a receiver
// only for uniformity — nothing about those frames varies by radio, as their
// own doc comments say — so they produce bytes on any receiver at all,
// including this one. The containment is the GATE: a zero dialect's
// AllowedCommand refuses even those frames, which is what keeps an
// unconfigured dialect from putting a single byte on a wire.
func RunZeroValue(t *testing.T) {
	t.Helper()

	var zero cat.Dialect

	if zero.Configured() {
		t.Fatal("the zero cat.Dialect reports Configured() == true — every refusal below rests on it not being configured")
	}
	if got := zero.MTForm(); got != cat.MTFormUnspecified {
		t.Errorf("zero dialect: MTForm() = %v, want MTFormUnspecified", got)
	}
	if min, max, err := zero.MTAnswerBounds(); err == nil {
		t.Errorf("zero dialect: MTAnswerBounds() = (%d, %d), nil — an unconfigured dialect must return an ERROR, never plausible zeros a caller would build a frame geometry from", min, max)
	} else if min != 0 || max != 0 {
		t.Errorf("zero dialect: MTAnswerBounds() returned (%d, %d) alongside its error, want (0, 0)", min, max)
	}

	// The data accessors: empty, false, and refusing.
	if zero.CATID() != "" {
		t.Errorf("zero dialect: CATID() = %q, want empty", zero.CATID())
	}
	if n := len(zero.EXAddresses()); n != 0 {
		t.Errorf("zero dialect: EXAddresses() returned %d addresses, want none", n)
	}
	if zero.ValidMode(cat.ModeUSB) {
		t.Error("zero dialect: ValidMode(ModeUSB) is true — it knows no modes at all")
	}
	if _, ok := zero.ModeByName("USB"); ok {
		t.Error("zero dialect: ModeByName(\"USB\") resolved — it has no mode table")
	}
	if _, err := zero.ParseMode('2'); err == nil {
		t.Error("zero dialect: ParseMode('2') succeeded — it knows no modes at all")
	}
	if _, err := zero.ParseSlot("001"); err == nil {
		t.Error("zero dialect: ParseSlot(\"001\") succeeded — it has no slot space")
	}

	// Every FALLIBLE builder must refuse, and return the zero Command with
	// its error (the contract command.go documents for all of them).
	slotless := cat.Slot{}
	builders := []struct {
		what string
		cmd  cat.Command
		err  error
	}{}
	add := func(what string, cmd cat.Command, err error) {
		builders = append(builders, struct {
			what string
			cmd  cat.Command
			err  error
		}{what, cmd, err})
	}
	mrCmd, mrErr := zero.BuildMRRead(slotless)
	add("BuildMRRead", mrCmd, mrErr)
	mtrCmd, mtrErr := zero.BuildMTRead(slotless)
	add("BuildMTRead", mtrCmd, mtrErr)
	mcCmd, mcErr := zero.BuildMCSet(slotless)
	add("BuildMCSet", mcCmd, mcErr)
	mtsCmd, mtsErr := zero.BuildMTSet(slotless, false, "AB")
	add("BuildMTSet", mtsCmd, mtsErr)
	mtcCmd, mtcErr := zero.BuildMTSetCombined(cat.MemoryData{}, "AB")
	add("BuildMTSetCombined", mtcCmd, mtcErr)
	mwCmd, mwErr := zero.BuildMWSet(cat.MemoryData{})
	add("BuildMWSet", mwCmd, mwErr)
	exCmd, exErr := zero.BuildEXRead(cat.EXAddress{P1: 1, P2: 1, P3: 1})
	add("BuildEXRead", exCmd, exErr)

	for _, b := range builders {
		if b.err == nil {
			t.Errorf("zero dialect: %s SUCCEEDED, emitting %q — an unconfigured dialect must build nothing", b.what, b.cmd.Bytes())
			continue
		}
		if !b.cmd.IsZero() {
			t.Errorf("zero dialect: %s returned a non-zero Command alongside its error; every fallible builder returns the zero Command", b.what)
		}
	}

	// Every dialect-dependent PARSER must refuse too. ParseIDAnswer is
	// deliberately absent: the reference does not constrain the ID body
	// against the receiver, so id.go documents that parser as
	// radio-independent, and it reads a well-formed ID answer on any
	// receiver.
	if _, _, _, err := zero.ParseMTAnswer([]byte("MT0011AB;")); err == nil {
		t.Error("zero dialect: ParseMTAnswer accepted a short-form frame — it has declared no form to parse one against")
	}
	if _, _, err := zero.ParseMTAnswerCombined([]byte("MT001014250000-001010211001" + "0" + "AB    " + ";")); err == nil {
		t.Error("zero dialect: ParseMTAnswerCombined accepted a combined frame — it has declared no form to parse one against")
	}
	if _, err := zero.ParseMRAnswer([]byte("MR001014250000-001010211001;")); err == nil {
		t.Error("zero dialect: ParseMRAnswer accepted a memory answer — it has no slot space to attribute the channel to")
	}

	// THE GATE. Every frame offered, including the ones this very dialect's
	// receiver-only builders emitted, and a well-formed frame of every
	// command the gate knows.
	offered := [][]byte{
		zero.BuildIDRead().Bytes(),
		zero.BuildAISet(false).Bytes(),
		zero.BuildAISet(true).Bytes(),
		zero.BuildMCRead().Bytes(),
		[]byte("ID0800;"),
		[]byte("AI0;"),
		[]byte("MC001;"),
		[]byte("MR001;"),
		[]byte("MT001;"),
		[]byte("MT0011AB;"),
		[]byte("EX010101;"),
		[]byte("MW001014250000-001010211001;"),
	}
	refused := 0
	for _, frame := range offered {
		if len(frame) == 0 {
			t.Error("zero dialect: one of its receiver-only builders produced no bytes — those three emit a fixed frame on any receiver, and this suite's containment claim is about the GATE refusing them, not about them not existing")
			continue
		}
		if zero.AllowedCommand(frame) {
			t.Errorf("zero dialect: its gate ADMITTED %q — an unconfigured dialect must authorise nothing, or a program holding one could put bytes on a wire to a radio it cannot describe", frame)
			continue
		}
		refused++
	}
	if refused != len(offered) {
		t.Errorf("zero dialect: %d of %d offered frames were refused", refused, len(offered))
	}
	if refused == 0 {
		t.Error("zero dialect: nothing was offered to the gate at all — this check would pass on a gate that admits everything")
	}

	t.Logf("zero dialect: %d fallible builders refused, %d frames refused at the gate", len(builders), refused)
}
