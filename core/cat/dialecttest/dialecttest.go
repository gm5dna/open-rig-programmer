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
	"fmt"
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
		t:           t,
		d:           d,
		frames:      map[string]int{},
		refusals:    map[string]int{},
		acceptances: map[string]int{},
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

	// acceptances counts the ACCEPTANCES a check needed to see, per kind,
	// and it is the third map for the same reason the second exists.
	//
	// A check whose whole content is "this dialect must NOT refuse X" fails
	// silently when it never runs: no error is exactly what passing looks
	// like. The five-state arm of checkToneStateDomain was in that state —
	// three required refusal counters on the three-state side, none at all on
	// the five-state side (adversarial review, finding L4) — and its early
	// returns mean not running is a reachable outcome, not a hypothetical.
	// Kept apart from frames because these are not per-builder frame counts
	// and their failure message has to say something else.
	acceptances map[string]int

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
	case cat.MTFormShort, cat.MTFormCombined, cat.MTFormShortNoDisplay:
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
	case cat.MTFormShortNoDisplay:
		if min != max {
			r.t.Errorf("%s: MTAnswerBounds() = (%d, %d) under MTFormShortNoDisplay — this form's frame length is EXACT (no display byte, a fixed-width tag field), so its bounds must be equal", r.name(), min, max)
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

	// BuildMCRead itself has no dialect data to validate ("MC;" is fixed,
	// reference: "Read: MC;"), so it builds successfully even under
	// MCSelectsUnsupported (core/cat/mc.go's own doc comment: the FTX-1's
	// real MC answer has a different shape entirely, so BuildMCRead is not
	// changed to refuse — ParseMCAnswer and the gate are the two places
	// that actually stop it being mistaken for this codec's own shape).
	// checkFrame's own "its own gate admits what its own builder built"
	// invariant would therefore be WRONG here: under MCSelectsUnsupported
	// the gate correctly refuses "MC;" (allowlist.go's validMCCommand),
	// and asserting agreement would demand the opposite of what
	// MCSelectsUnsupported means. So this checks the MATCHING invariant
	// instead: build clean, gate REFUSES, under that one policy only.
	mcRead := r.d.BuildMCRead().Bytes()
	if r.d.MCSelects() == cat.MCSelectsUnsupported {
		if r.d.AllowedCommand(mcRead) {
			r.t.Errorf("%s: its own gate ADMITTED %q under MCSelectsUnsupported — this dialect declares no MC support at all, so no MC frame (read included) may reach the wire", r.name(), mcRead)
		} else {
			r.refusals["MC read refused at the gate under MCSelectsUnsupported"]++
		}
	} else {
		r.checkFrame("MC read", mcRead)
	}

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
	return fmt.Sprintf("%03d", n)
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

	seen := make(map[string]bool)
	add := func(s cat.Slot, err error) {
		if err == nil && !seen[s.Wire()] {
			r.slots = append(r.slots, s)
			seen[s.Wire()] = true
		}
	}

	for n := 0; n <= 999; n++ {
		if s, err := r.d.ParseSlot(threeDigits(n)); err == nil {
			add(s, nil)
		}
	}
	// WIDTH-AGNOSTIC SWEEP, added for the FTX-1's 5-digit slot space
	// (SlotSpace.SlotDigits): the three-digit sweep above only ever
	// produces 3-byte wire forms, so a wider dialect would have ZERO
	// memory or 60m/5MHz slots discovered by it at all. MemorySlot and
	// SixtyMSlot render THIS dialect's own declared width, so sweeping
	// through them directly reaches every dialect's memory and 60m/5MHz
	// bank regardless of width. Deduped against the sweep above (the
	// add closure), so a registered (3-digit) dialect's r.slots is
	// unchanged: MemorySlot(n) there already IS threeDigits(n) for every
	// n the sweep above found.
	for n := 0; n <= 9999; n++ {
		add(r.d.MemorySlot(n))
	}
	for n := 1; n <= 500; n++ {
		add(r.d.SixtyMSlot(n))
	}
	// PMSPairs is capped at 9 under PMSFormToken (the wire form is one
	// digit between 'P' and 'L'/'U'), but wider under PMSFormDashToken
	// (the FTX-1's own 50 pairs) or PMSFormNumeric — so this sweeps until
	// PMSSlot itself stops building, exactly as checkPMSSlotForm's own
	// positive sweep does (maxSweptPMSPairs is its runaway guard, reused
	// here for the same reason).
	for pair := 1; pair <= maxSweptPMSPairs; pair++ {
		if _, err := r.d.PMSSlot(pair, false); err != nil {
			break
		}
		add(r.d.PMSSlot(pair, false))
		add(r.d.PMSSlot(pair, true))
	}
	if s := r.d.EMGSlot(); s.Wire() != "" {
		add(s, nil)
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
		case cat.MTFormShortNoDisplay:
			r.checkNoDisplayMTSets(s)
		}
	}

	// An MR answer cannot be round-tripped from a built frame: BuildMRRead
	// emits a READ request, and only a radio can produce the 28-byte answer
	// that carries the data back. That half is covered in-package against
	// captured frames, and is out of this suite's reach by design.

	r.checkMCSendDomain()
	r.checkMTReadDomain()
	r.checkPMSSlotForm()
	r.checkToneStateDomain()
	r.checkOverlongMTFrameIsRefused()
}

// checkMCSendDomain requires BuildMCSet and the gate's own verdict on
// "MC"+s.Wire()+";" to AGREE for every slot this dialect classifies — both
// admit, or both refuse — and requires that agreement to match the SEND-side
// policy this dialect declares (MCSelects): under MCSelectsMemoryPMS every
// 60m/EMG slot is refused by both while every memory/PMS slot is admitted by
// both; under MCSelectsAll every slot this dialect classifies is admitted by
// both.
//
// The loop above already proves half of this: every "MC set" frame the
// builder DOES produce is offered to checkFrame, which asserts the gate
// admits it. It is silent when the builder REFUSES, which is exactly the gap
// a builder or a gate reading the wrong policy value would hide behind — a
// gate that admitted everything, or a builder that refused everything, would
// leave no evidence in that loop alone. This function is what turns the
// refusal into a checked property, with its own non-vacuity counter
// (checkNonVacuity) so a dialect with no 60m/EMG slot in its space cannot
// satisfy that counter by having nothing to offer.
func (r *conformanceRun) checkMCSendDomain() {
	r.t.Helper()

	policy := r.d.MCSelects()
	for _, s := range r.slots {
		cmd, buildErr := r.d.BuildMCSet(s)
		builderOK := buildErr == nil
		gateOK := r.d.AllowedCommand([]byte("MC" + s.Wire() + ";"))

		if builderOK != gateOK {
			r.t.Errorf("%s: MC send domain disagreement for slot %q under %v — BuildMCSet admits=%v, gate admits=%v; a builder and a gate that disagree about the send domain mean one of them is not reading this dialect's own MCSelects", r.name(), s.Wire(), policy, builderOK, gateOK)
			continue
		}
		if !builderOK && !cmd.IsZero() {
			r.t.Errorf("%s: BuildMCSet returned a non-zero Command alongside its refusal for slot %q; every fallible builder returns the zero Command", r.name(), s.Wire())
		}

		if policy == cat.MCSelectsUnsupported {
			if builderOK {
				r.t.Errorf("%s: BuildMCSet ADMITTED slot %q under MCSelectsUnsupported — this dialect declares NO MC support at all (its real MC frame has a shape this codec cannot build), so every slot must be refused", r.name(), s.Wire())
				continue
			}
			r.refusals["MC send refused for every slot under MCSelectsUnsupported"]++
			continue
		}

		wide := s.Is60m() || s.IsEMG()
		switch {
		case wide && policy == cat.MCSelectsMemoryPMS:
			if builderOK {
				r.t.Errorf("%s: BuildMCSet ADMITTED slot %q under %v — its MC legend does not print the 5xx or EMG banks, and an MC Set recalls the channel on the radio", r.name(), s.Wire(), policy)
				continue
			}
			r.refusals["MC send refused for 60m/EMG under MCSelectsMemoryPMS"]++
		case wide:
			if !builderOK {
				r.t.Errorf("%s: BuildMCSet refused slot %q (%v) under %v — 60m and EMG are in the MC send domain under MCSelectsAll", r.name(), s.Wire(), buildErr, policy)
			}
		case s.IsMemory() || s.IsPMS():
			if !builderOK {
				r.t.Errorf("%s: BuildMCSet refused slot %q (%v) under %v — memory and PMS are always in the MC send domain", r.name(), s.Wire(), buildErr, policy)
			}
		}
	}
}

// checkPMSSlotForm holds this dialect's PMS pairs to the WIRE FORM it
// declares, in BOTH directions.
//
// The axis is not a policy like MCSelects or MTReadSlots: it decides WHICH
// BYTES a builder emits. Under cat.PMSFormToken pair k is "P<k><L|U>" and
// no decimal wire number is ever a PMS slot; under cat.PMSFormNumeric pair
// k is a pair of consecutive decimal channel numbers and the token form is
// not a wire form this radio has at all. Getting it wrong is a WRITE
// defect: cat.Dialect.writableSlot admits every PMS slot, so a numeric-PMS
// radio classifying "P1L" as PMS would have "MW P1L…;" built for it and
// admitted by its own gate — a frame its manual never prints.
//
// So both halves are asserted here rather than only the positive one: what
// PMSSlot BUILDS must round-trip through this dialect's own ParseSlot as a
// PMS slot, and every form of the OTHER shape must be refused. The refusal
// counter is keyed by form so that a dialect of either kind must be SEEN to
// refuse the other's forms, never merely to have none to offer.
// pmsMatchesDeclaredTokenShape reports whether wire is a token-shaped PMS
// slot IN form's OWN shape — "P<n><L|U>" (three bytes) under
// cat.PMSFormToken, "P-<nn><L|U>" (five bytes) under cat.PMSFormDashToken.
// Neither form's shape is mistaken for the other's: PMSFormToken's own
// three-byte form never has a hyphen for pmsMatchesDeclaredTokenShape to
// even look at, and PMSFormDashToken's own five-byte form is never
// three bytes long, so a dialect declaring one cannot pass by having a
// wire form shaped like the other's.
func pmsMatchesDeclaredTokenShape(form cat.PMSSlotForm, wire string) bool {
	switch form {
	case cat.PMSFormToken:
		return len(wire) == 3 && wire[0] == 'P' && (wire[2] == 'L' || wire[2] == 'U')
	case cat.PMSFormDashToken:
		return len(wire) == 5 && wire[0] == 'P' && wire[1] == '-' && (wire[4] == 'L' || wire[4] == 'U')
	default:
		return false
	}
}

func (r *conformanceRun) checkPMSSlotForm() {
	r.t.Helper()

	form := r.d.PMSForm()
	// A dialect with no pairs declares no form (V15 requires one only when
	// PMSPairs > 0), and has nothing for this check to hold.
	if _, err := r.d.PMSSlot(1, false); err != nil {
		return
	}

	// The positive half: every pair this dialect can build must come back
	// from its own ParseSlot as a PMS slot, in the declared shape.
	//
	// IT SWEEPS UNTIL THIS DIALECT'S OWN PMSSlot STOPS BUILDING, not to a
	// fixed nine. pmsCap() clamps to nine under PMSFormToken ONLY — the pair
	// number is one ASCII wire byte there, which TestV3_PairBoundIsFormAware
	// pins — so a numeric dialect may legally declare more, and a fixed
	// bound left every pair past the ninth built by PMSSlot, classified by
	// classifySlot and never seen by this suite (Stage 0 close review, seat
	// 2 LOW-3). maxSweptPMSPairs is a runaway guard on a loop whose real
	// bound is the receiver, not a domain: a numeric pair costs two channels
	// inside 000-999, so no dialect can offer 500 of them.
	for pair := 1; pair <= maxSweptPMSPairs; pair++ {
		if _, err := r.d.PMSSlot(pair, false); err != nil {
			break
		}
		for _, upper := range []bool{false, true} {
			s, err := r.d.PMSSlot(pair, upper)
			if err != nil {
				continue
			}
			wantShape := form == cat.PMSFormToken || form == cat.PMSFormDashToken
			if wantShape != pmsMatchesDeclaredTokenShape(form, s.Wire()) {
				r.t.Errorf("%s: PMSSlot(%d, %t) built %q under %v — the token form is \"P<n><L|U>\", the dash-token form is \"P-<nn><L|U>\", and the numeric form is a decimal channel number; a builder emitting a shape other than its OWN declared one puts bytes on the wire this dialect's manual does not print", r.name(), pair, upper, s.Wire(), form)
				continue
			}
			back, err := r.d.ParseSlot(s.Wire())
			if err != nil {
				r.t.Errorf("%s: its own ParseSlot refused %q, which its own PMSSlot(%d, %t) built: %v", r.name(), s.Wire(), pair, upper, err)
				continue
			}
			if !back.IsPMS() {
				r.t.Errorf("%s: ParseSlot(%q) classified its own PMSSlot(%d, %t) output as something other than PMS — the constructor and the classifier disagree", r.name(), s.Wire(), pair, upper)
			}
		}
	}

	// The negative half: every form of the OTHER shape is refused.
	switch form {
	case cat.PMSFormNumeric:
		for pair := 1; pair <= 9; pair++ {
			for _, suffix := range []byte{'L', 'U'} {
				wire := string([]byte{'P', byte('0' + pair), suffix})
				s, err := r.d.ParseSlot(wire)
				if err == nil && s.IsPMS() {
					r.t.Errorf("%s: ParseSlot(%q) = a PMS slot under %v — this dialect's pairs are decimal channel numbers, so no token form is a slot it has", r.name(), wire, form)
					continue
				}
				r.refusals["token PMS form refused under PMSFormNumeric"]++
			}
		}
	case cat.PMSFormToken:
		// Every three-digit form this dialect classifies at all must be a
		// memory, 60m, EMG or none slot — never PMS, because under the
		// token form no decimal number is one.
		//
		// THE COUNTER SAYS THE SWEEP RAN, AND NOTHING MORE, because nothing
		// more is true. It was called "decimal PMS form refused under
		// PMSFormToken" and incremented only on the error branch, which is
		// most of 000-999 on any dialect: it was non-zero before this axis
		// existed and would stay non-zero if the whole numeric arm of
		// classifySlot were deleted (adversarial review, finding L5). There
		// is no honest refusal to count on this side — a token dialect has no
		// numeric PMS range to refuse, and that ABSENCE is exactly what the
		// IsPMS() assertion below states — so the counter is named for the
		// only thing it witnesses: every three-digit form in the space is
		// declined as PMS, whether by not classifying at all or by
		// classifying as some other bank, and it counts them all. Its twin
		// "token PMS form refused under PMSFormNumeric" IS a genuine refusal
		// count and keeps its name.
		for n := 0; n <= 999; n++ {
			wire := threeDigits(n)
			r.refusals["three-digit form declined as PMS under PMSFormToken"]++
			s, err := r.d.ParseSlot(wire)
			if err != nil {
				continue // not a slot this dialect has, in any bank
			}
			if s.IsPMS() {
				r.t.Errorf("%s: ParseSlot(%q) = a PMS slot under %v — this dialect's pairs are the \"P<n><L|U>\" token, so no decimal channel number is one", r.name(), wire, form)
			}
		}
	case cat.PMSFormDashToken:
		// The single-digit token ("P1L".."P9U") is NOT this dialect's own
		// shape — its own pairs are "P-01L".."P-50U" — so ParseSlot must
		// never classify one as PMS.
		for pair := 1; pair <= 9; pair++ {
			for _, suffix := range []byte{'L', 'U'} {
				wire := string([]byte{'P', byte('0' + pair), suffix})
				s, err := r.d.ParseSlot(wire)
				if err == nil && s.IsPMS() {
					r.t.Errorf("%s: ParseSlot(%q) = a PMS slot under %v — this dialect's pairs are the dash-token \"P-<nn><L|U>\" shape, so the single-digit token is not one", r.name(), wire, form)
					continue
				}
				r.refusals["single-digit token PMS form refused under PMSFormDashToken"]++
			}
		}
	default:
		// A THIRD PMS form would take this whole check out of the suite
		// silently, which is the hazard the counters above exist for
		// (Stage 0 close review, seat 2 LOW-4). Production switches on this
		// axis all refuse; this one is the suite's own, and says so here.
		r.t.Errorf("%s: declares PMS form %v, which this check does not enumerate — add its arm here, or the wire form of a whole class of slots is conformed by nothing", r.name(), form)
	}
}

// maxSweptPMSPairs bounds checkPMSSlotForm's positive sweep. It is a
// runaway guard, NOT this suite's opinion about how many pairs a dialect may
// declare: the loop's real bound is where the receiver's own PMSSlot stops
// building. A numeric pair occupies two consecutive channels inside the
// three-digit space, so 500 pairs is already more than any slot space can
// hold, and reaching this bound means PMSSlot answered for a pair no wire
// form could carry.
const maxSweptPMSPairs = 500

// checkToneStateDomain holds this dialect to its declared P8 domain in
// THREE places, which is how many places decide it.
//
// A DCS state ('3' or '4') must be refused by a three-state dialect at
// BuildMWSet, at BuildMTSetCombined AND at AllowedCommand — it cannot be
// BUILT by any route, and a frame forged elsewhere cannot get past the gate
// either. Widening only the parse site would have let a MemoryData carrying
// CTCSSState('3') be built, admitted and SENT to a radio whose manual prints
// P8 0/1/2 only, which is what makes this a write-direction check rather
// than a codec nicety.
//
// The forged frame is spliced from one this SAME dialect built and its own
// gate admits, so the only difference the gate can be reacting to is P8.
func (r *conformanceRun) checkToneStateDomain() {
	r.t.Helper()

	domain := r.d.ToneStates()
	if len(r.emittable) == 0 || len(r.slots) == 0 {
		return
	}
	ctcss, shift, ok := r.mustParseStates()
	if !ok {
		return
	}
	mem, found := r.firstWritableSlot(r.emittable[0], ctcss, shift)
	if !found {
		return // checkMemoryWrites has already reported this
	}
	base := recordFor(mem, r.emittable[0], r.d.MWWriteKind(), 0, ctcss, shift)

	clean, err := r.d.BuildMWSet(base)
	if err != nil {
		r.t.Errorf("%s: BuildMWSet refused a plain CTCSS-off record for slot %q: %v", r.name(), mem.Wire(), err)
		return
	}
	if !r.d.AllowedCommand(clean.Bytes()) {
		r.t.Errorf("%s: its own gate refused its own MW frame %q", r.name(), clean.Bytes())
		return
	}

	// ToneStatesSix (the FTX-1's own P8 domain, dialectconfig.go) adds a
	// THIRD byte, '5' ("REV TONE"), beyond the two every other widened
	// domain shares the bytes of ('3'/'4', reused here as raw wire values
	// — cat.CTCSSDCSEncDec/cat.CTCSSDCSEnc are simply the names those
	// SAME BYTES carry under ToneStatesCTCSSAndDCS; ToneStatesSix's own
	// '4' means something else entirely, "PR FREQ" not "DCS ENC", but the
	// byte value is what this check round-trips).
	states := []cat.CTCSSState{cat.CTCSSDCSEncDec, cat.CTCSSDCSEnc}
	if domain == cat.ToneStatesSix {
		states = append(states, cat.CTCSSState('5'))
	}
	for _, state := range states {
		m := base
		m.CTCSS = state

		_, mwErr := r.d.BuildMWSet(m)
		forged := append([]byte(nil), clean.Bytes()...)
		forged[ctcssOffsetFor(len(forged))] = state.Wire()
		gateOK := r.d.AllowedCommand(forged)

		mtBuilt := false
		if r.d.MTForm() == cat.MTFormCombined {
			combined := m
			combined.Kind = cat.CombinedMTSetKind
			cmd, mtErr := r.buildCombined(combined, "TAG", true)
			mtBuilt = mtErr == nil && !cmd.IsZero()
		}

		switch domain {
		case cat.ToneStatesCTCSS:
			if mwErr == nil {
				r.t.Errorf("%s: BuildMWSet built an MW frame carrying P8 %q under %v — this radio's legend prints 0/1/2 only", r.name(), state.Wire(), domain)
			} else {
				r.refusals["DCS state refused at BuildMWSet under ToneStatesCTCSS"]++
			}
			if gateOK {
				r.t.Errorf("%s: its own gate ADMITTED the forged MW frame %q, whose P8 is %q, under %v", r.name(), forged, state.Wire(), domain)
			} else {
				r.refusals["DCS state refused at the gate under ToneStatesCTCSS"]++
			}
			if r.d.MTForm() == cat.MTFormCombined {
				if mtBuilt {
					r.t.Errorf("%s: BuildMTSetCombined built a combined MT frame carrying P8 %q under %v", r.name(), state.Wire(), domain)
				} else {
					r.refusals["DCS state refused at BuildMTSetCombined under ToneStatesCTCSS"]++
				}
			}
		case cat.ToneStatesCTCSSAndDCS:
			// EVERY leg of this arm is COUNTED, for the reason the
			// three-state arm's refusal counters exist. This arm asserts
			// absences of errors, so an arm that never ran looks exactly like
			// an arm that ran and found nothing wrong: the early returns
			// above, or a fixture with no writable slot, would have taken the
			// whole five-state axis out of the suite silently. The adversarial
			// review recorded that asymmetry as finding L4;
			// checkConformanceCoverage requires these three by name.
			if mwErr != nil {
				r.t.Errorf("%s: BuildMWSet refused P8 %q under %v (%v) — this radio's legend prints it", r.name(), state.Wire(), domain, mwErr)
			} else {
				r.acceptances["DCS state accepted at BuildMWSet under ToneStatesCTCSSAndDCS"]++
			}
			if !gateOK {
				r.t.Errorf("%s: its own gate refused the MW frame %q, whose P8 %q its own legend prints, under %v", r.name(), forged, state.Wire(), domain)
			} else {
				r.acceptances["DCS state accepted at the gate under ToneStatesCTCSSAndDCS"]++
			}
			// The combined MT builder is a SEPARATE consultation of the same
			// domain, so it gets its own assertion in this direction too: a
			// site left on the package-level three-state parser would refuse
			// here while MW happily built the frame.
			if r.d.MTForm() == cat.MTFormCombined {
				if !mtBuilt {
					r.t.Errorf("%s: BuildMTSetCombined refused P8 %q under %v — this radio's legend prints it, and MW built it", r.name(), state.Wire(), domain)
				} else {
					r.acceptances["DCS state accepted at BuildMTSetCombined under ToneStatesCTCSSAndDCS"]++
				}
			}
			// And the parse direction: a '3' or '4' must come back as the
			// state rather than as an error.
			answer := append([]byte("MR"), forged[2:]...)
			back, err := r.d.ParseMRAnswer(answer)
			if err != nil {
				r.t.Errorf("%s: ParseMRAnswer(%q) refused P8 %q under %v: %v", r.name(), answer, state.Wire(), domain, err)
			} else if back.CTCSS != state {
				r.t.Errorf("%s: ParseMRAnswer decoded P8 %q as %v, want %v", r.name(), state.Wire(), back.CTCSS, state)
			} else {
				r.acceptances["DCS state decoded by ParseMRAnswer under ToneStatesCTCSSAndDCS"]++
			}
		case cat.ToneStatesSix:
			// Same shape as ToneStatesCTCSSAndDCS's own arm — every state
			// this domain names, byte '3' through '5', must be ACCEPTED at
			// every site that decides P8 — counted under this domain's own
			// name so a dispatch bug that silently reused the five-state
			// arm's logic (right answer, wrong counter) is still visible
			// to checkNonVacuity's ledger.
			if mwErr != nil {
				r.t.Errorf("%s: BuildMWSet refused P8 %q under %v (%v) — this radio's legend prints it", r.name(), state.Wire(), domain, mwErr)
			} else {
				r.acceptances["state accepted at BuildMWSet under ToneStatesSix"]++
			}
			if !gateOK {
				r.t.Errorf("%s: its own gate refused the MW frame %q, whose P8 %q its own legend prints, under %v", r.name(), forged, state.Wire(), domain)
			} else {
				r.acceptances["state accepted at the gate under ToneStatesSix"]++
			}
			if r.d.MTForm() == cat.MTFormCombined {
				if !mtBuilt {
					r.t.Errorf("%s: BuildMTSetCombined refused P8 %q under %v — this radio's legend prints it, and MW built it", r.name(), state.Wire(), domain)
				} else {
					r.acceptances["state accepted at BuildMTSetCombined under ToneStatesSix"]++
				}
			}
			answer := append([]byte("MR"), forged[2:]...)
			back, err := r.d.ParseMRAnswer(answer)
			if err != nil {
				r.t.Errorf("%s: ParseMRAnswer(%q) refused P8 %q under %v: %v", r.name(), answer, state.Wire(), domain, err)
			} else if back.CTCSS != state {
				r.t.Errorf("%s: ParseMRAnswer decoded P8 %q as %v, want %v", r.name(), state.Wire(), back.CTCSS, state)
			} else {
				r.acceptances["state decoded by ParseMRAnswer under ToneStatesSix"]++
			}
		default:
			// A THIRD tone-state domain would silently take the whole P8
			// write-direction check out of the suite (seat 2 LOW-4). V16
			// refuses an undeclared domain in production; this is the
			// suite's own enumeration saying it is not complete.
			r.t.Errorf("%s: declares tone-state domain %v, which this check does not enumerate — add its arm here, or nothing conforms P8 on radios of that kind", r.name(), domain)
		}
	}
}

// ctcssOffsetFor returns THIS DIALECT'S OWN byte offset for P8 (CTCSS) in a
// short-form MR/MW frame of mwFrameLen bytes — position 24 (1-indexed) of
// the registered 28-byte family, but SHIFTED by however many bytes
// narrower or wider that dialect's P2 frequency field is (core/cat's
// Lift-Y MemoryFrameLen/MemoryFreqDigits axes, commit 99cdaf9): the
// ft2000/ftdx9000 family's 27-byte, 8-digit-P2 frame puts it one byte
// earlier, at 23.
//
// core/cat holds its own field-block offsets unexported, so this package
// cannot read them directly; mwFrameLen is the one length it CAN observe
// (a frame the dialect itself just built), and every field from P3 onward
// is a fixed distance from the frame's OWN end regardless of P2's width —
// P8 is always 5 bytes before the final ';' — so deriving the offset from
// mwFrameLen rather than a hardcoded 23 is correct for both the registered
// family and the 27-byte one. The splice is taken from a frame the dialect
// itself built and its own gate admitted, so a wrong offset would corrupt
// some other field and show up as a refusal on the FIVE-state arm rather
// than passing silently.
func ctcssOffsetFor(mwFrameLen int) int { return mwFrameLen - 5 }

// checkMTReadDomain is checkMCSendDomain's counterpart for MT: BuildMTRead
// against the gate's own verdict on "MT"+s.Wire()+";", held to this
// dialect's declared MTReadSlots rather than MC's MCSelects — the two
// policies are independent (a radio may narrow one legend and not the
// other), so each gets its own walk and its own non-vacuity counter.
func (r *conformanceRun) checkMTReadDomain() {
	r.t.Helper()

	policy := r.d.MTReadSlots()
	for _, s := range r.slots {
		cmd, buildErr := r.d.BuildMTRead(s)
		builderOK := buildErr == nil
		gateOK := r.d.AllowedCommand([]byte("MT" + s.Wire() + ";"))

		if builderOK != gateOK {
			r.t.Errorf("%s: MT read domain disagreement for slot %q under %v — BuildMTRead admits=%v, gate admits=%v; a builder and a gate that disagree about the read domain mean one of them is not reading this dialect's own MTReadSlots", r.name(), s.Wire(), policy, builderOK, gateOK)
			continue
		}
		if !builderOK && !cmd.IsZero() {
			r.t.Errorf("%s: BuildMTRead returned a non-zero Command alongside its refusal for slot %q; every fallible builder returns the zero Command", r.name(), s.Wire())
		}

		wide := s.Is60m() || s.IsEMG()
		switch {
		case wide && policy == cat.MTReadsMemoryPMS:
			if builderOK {
				r.t.Errorf("%s: BuildMTRead ADMITTED slot %q under %v — its MT block's slot legend prints neither the 5xx nor the EMG bank, which MR reads instead", r.name(), s.Wire(), policy)
				continue
			}
			r.refusals["MT read refused for 60m/EMG under MTReadsMemoryPMS"]++
		case wide:
			if !builderOK {
				r.t.Errorf("%s: BuildMTRead refused slot %q (%v) under %v — 60m and EMG are in the MT read domain under MTReadsReadable", r.name(), s.Wire(), buildErr, policy)
			}
		case s.IsMemory() || s.IsPMS():
			if !builderOK {
				r.t.Errorf("%s: BuildMTRead refused slot %q (%v) under %v — memory and PMS are always in the MT read domain", r.name(), s.Wire(), buildErr, policy)
			}
		}
	}
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
		txClarOff := txClarOffsetFor(len(forged))
		if got := forged[txClarOff]; got != '0' {
			r.t.Errorf("%s: its own MW Set carries %q at position 21 under %v, want '0' — the printed-fixed byte is not being written where the frame puts it", r.name(), got, policy)
		}
		forged[txClarOff] = '1'
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
		if got := frame[txClarOffsetFor(len(frame))]; got != '1' {
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
	// refusal entirely unexercised from outside core/cat. This is the S0-MEM
	// lane review's LOW-6 finding; checkCombinedMemoryP5 below is the fix.
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

	// A short-form MW Set this dialect actually built, consulted ONLY for
	// its length: the combined form's own frame carries a variable-width
	// tag field after the shared block, so its length cannot stand in for
	// txClarOffsetFor's mwFrameLen — see that function's doc comment. The
	// shared field block sits at the same offsets from the START in both
	// forms, and a plain (TxClar false) record is legal to build under
	// either MemoryP5Policy, so this never fails for a reason unrelated to
	// the axis under test here.
	ref, refErr := r.d.BuildMWSet(recordFor(writable, r.emittable[0], r.d.MWWriteKind(), 0, ctcss, shift))
	if refErr != nil {
		r.t.Errorf("%s: BuildMWSet with TxClar false was refused (%v) — needed only to measure this dialect's own frame-block offsets", r.name(), refErr)
		return
	}
	mwFrameLen := len(ref.Bytes())

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
		if got := frame[txClarOffsetFor(mwFrameLen)]; got != '1' {
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

// txClarOffsetFor returns THIS DIALECT'S OWN byte offset for P5 (the TX
// clarifier flag cat.MemoryP5Policy governs) in a short-form MR/MW frame of
// mwFrameLen bytes — position 21 (1-indexed) of the registered 28-byte
// family, SHIFTED the same way ctcssOffsetFor's doc comment describes for
// P8: always 8 bytes before the frame's own final ';', so deriving it from
// mwFrameLen is correct for both the registered family and the 27-byte
// ft2000/ftdx9000 one (core/cat's Lift-Y axes, commit 99cdaf9).
//
// It is restated here rather than imported because core/cat's offsets are
// unexported, which is the condition this package works under. What it
// must NOT become is a second opinion: the checks above assert that this
// dialect's own builder puts the flag HERE, which is what would fail if
// core/cat ever moved the field.
//
// The SAME formula holds for the combined MT form's own frame, whose
// mwFrameLen argument a caller must supply from a SHORT-form MW frame this
// dialect built (never the combined frame's own length, which carries a
// variable-width tag field after this offset and would corrupt the
// arithmetic) — the shared field block sits at identical offsets from the
// START in both forms, and mwFrameLen is only ever used to recover the one
// axis (P2's digit width) that varies it.
func txClarOffsetFor(mwFrameLen int) int { return mwFrameLen - 8 }

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

// checkNoDisplayMTSets drives MTFormShortNoDisplay's own Set builder
// (BuildMTSetNoDisplay) over slot s and round-trips every frame it
// produces.
//
// EVERY ACCEPTED TAG MUST ROUND-TRIP EXACTLY, for the combined form's own
// reason (checkCombinedMTSets' doc comment): this form's builder REFUSES a
// tag ending in the fill byte outright, exactly like the combined form's
// does, so there is no short-form-style partial-trim tolerance to allow —
// the tag field is fixed-width and TagFill-padded on the way out, and the
// parser trims exactly that padding back off on the way in, never more.
func (r *conformanceRun) checkNoDisplayMTSets(s cat.Slot) {
	r.t.Helper()

	if cmd, err := r.d.BuildMTSetNoDisplay(s, ""); err == nil {
		frame := cmd.Bytes()
		r.checkFrame("MT set no-display (cleared)", frame)
		r.checkMTFrameLength("MT set no-display (cleared)", frame, true)
		r.checkNoDisplayRoundTrip(frame, s, "")
		r.keepOwnFormFrames(frame, true)
	}
	for _, tag := range conformanceTags {
		cmd, err := r.d.BuildMTSetNoDisplay(s, tag)
		if err != nil {
			// A tag wider than this dialect's field, one ending in the
			// fill byte, or a slot this dialect's MT write policy
			// refuses. Neither is this check's business; the
			// non-vacuity floor catches a builder refusing everything.
			continue
		}
		frame := cmd.Bytes()
		r.checkFrame("MT set no-display (tagged)", frame)
		r.checkMTFrameLength("MT set no-display (tagged)", frame, true)
		r.checkNoDisplayRoundTrip(frame, s, tag)
		r.keepOwnFormFrames(frame, false)
	}
}

// checkNoDisplayRoundTrip parses a frame checkNoDisplayMTSets' builder just
// produced and holds the result to the input — EXACTLY, per
// checkNoDisplayMTSets' own doc comment.
func (r *conformanceRun) checkNoDisplayRoundTrip(frame []byte, want cat.Slot, wantTag string) {
	r.t.Helper()

	slot, tag, err := r.d.ParseMTAnswerNoDisplay(frame)
	if err != nil {
		r.t.Errorf("%s: ParseMTAnswerNoDisplay(%q) = %v — a frame its own builder produced must parse", r.name(), frame, err)
		return
	}
	if slot.Wire() != want.Wire() {
		r.t.Errorf("%s: ParseMTAnswerNoDisplay(%q) returned slot %q, want %q", r.name(), frame, slot.Wire(), want.Wire())
	}
	if tag != wantTag {
		r.t.Errorf("%s: ParseMTAnswerNoDisplay(%q) returned tag %q, want %q exactly — this form's tag field is fixed-width and TagFill-padded, so its round trip has no partial-trim tolerance", r.name(), frame, tag, wantTag)
		return
	}
	if wantTag != "" {
		r.exactTagRoundTrips++
	}
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
//
// UNDER MTFormCombined THE PARSER CALLED MUST FOLLOW d.MTP11(), not always
// the display-less ParseMTAnswerCombined (S0-close review's MEDIUM-3
// finding): that parser refuses a P11TagDisplay dialect UNCONDITIONALLY,
// before it ever looks at length — "use the display-bearing
// builder/parser" — so on the FT-891-shaped fixture this check exercised no
// length rule at all, and its refusal counter passed on the wrong-API
// refusal instead. Each combined-form policy gets its OWN non-vacuity
// counter below, so a dispatch bug that silently reverted to one parser for
// both policies leaves the other's counter at zero.
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
	refusalKey := "over-long MT answer"
	switch r.d.MTForm() {
	case cat.MTFormShort:
		_, _, _, err = r.d.ParseMTAnswer(over)
	case cat.MTFormCombined:
		switch r.d.MTP11() {
		case cat.P11TagDisplay:
			_, _, _, err = r.d.ParseMTAnswerCombinedDisplay(over)
			refusalKey = "over-long MT answer (display-bearing)"
		default:
			_, _, err = r.d.ParseMTAnswerCombined(over)
			refusalKey = "over-long MT answer (display-less)"
		}
	case cat.MTFormShortNoDisplay:
		_, _, err = r.d.ParseMTAnswerNoDisplay(over)
	}
	if err == nil {
		r.t.Errorf("%s: its own answer parser ACCEPTED %q, one byte past the %d-byte top of the window it reports — the answer window is not deriving from this dialect", r.name(), over, r.mtMax)
	} else if strings.Contains(err.Error(), "use the display") {
		// THE REFUSAL MUST BE FOR LENGTH, NOT THE WRONG API: this is
		// exactly the vacuous shape MEDIUM-3 found — a dispatch bug (or the
		// pre-fix code, which always called the display-less parser) gets a
		// non-nil error here too, but it says nothing about the frame being
		// one byte too long.
		r.t.Errorf("%s: the over-long-frame refusal %q is the WRONG-API refusal, not a length refusal — this check called the parser for the WRONG P11 policy (this dialect declares %v)", r.name(), err, r.d.MTP11())
	} else {
		r.refusals[refusalKey]++
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

	// The CTCSS and shift wire spaces, exhaustively, through the PACKAGE-LEVEL
	// parsers. Both are byte-alias types, so a parser is how this suite gets a
	// value a builder will accept without hardcoding one.
	//
	// cat.ParseCTCSSState rather than r.d.ParseCTCSSState DELIBERATELY. Since
	// the P8 domain became dialect data there are two routes, and this matrix
	// wants the frozen three-state one: what it is written to catch is a
	// BuildMWSet that refuses a state every dialect accepts, and holding it to
	// '0'-'2' keeps that meaning identical on a three-state and a five-state
	// radio. The DOMAIN axis — that a five-state dialect accepts '3'/'4' and a
	// three-state one refuses them at all three deciding sites — is
	// checkToneStateDomain's, above, and is not restated here.
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
	case cat.MTFormShortNoDisplay:
		// Either sibling builder is "wrong" for this form; the short
		// form's is picked because it takes no slot data to construct a
		// call with (BuildMTSet checks its own Form first, before it ever
		// looks at the slot or tag it was handed).
		cmd, err = r.d.BuildMTSet(cat.Slot{}, false, "AB")
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
	case cat.MTFormShortNoDisplay:
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
		"ID read", "AI set (off)", "AI set (on)",
		"MR read", "MT read",
		"MW set", "MW set (CTCSS/shift matrix)", "MW set (clarifier endpoint)",
	}
	// "MC read"/"MC set" are meaningless requirements under
	// MCSelectsUnsupported: this dialect's real MC frame has no
	// representation in this codec at all, so BuildMCSet refuses for
	// EVERY slot by design and checkFixedFrames routes "MC read" through
	// a refusal counter instead of r.frames (see its own doc comment) —
	// requiring either here would fail every such dialect for behaving
	// exactly as it must.
	if r.d.MCSelects() != cat.MCSelectsUnsupported {
		required = append(required, "MC read", "MC set")
	}
	switch r.d.MTForm() {
	case cat.MTFormShort:
		required = append(required, "MT set short (tagged)", "MT set short (cleared)")
	case cat.MTFormCombined:
		required = append(required, "MT set combined (tagged)", "MT set combined (cleared)")
	case cat.MTFormShortNoDisplay:
		required = append(required, "MT set no-display (tagged)", "MT set no-display (cleared)")
	}
	// The P5 arm this dialect's own policy selects. Both arms are counted,
	// because "the check ran" and "the check ran for THIS policy" are
	// different claims: a suite that only required the P5TxClar frame would
	// go green on a P5Fixed dialect whose whole arm had been skipped.
	requiredRefusals := []string{
		"wrong-form MT Set build",
		"wrong-form MT answer parse",
		"over-long MT frame at the gate",
		"clarifier past MaxAbsHz",
		"malformed frame at the gate",
	}
	// requiredAcceptances is the mirror list for checks whose content is that
	// this dialect must NOT refuse something. Empty for a dialect that has no
	// such check to satisfy; see conformanceRun.acceptances for why they
	// cannot be left to the two lists above.
	var requiredAcceptances []string
	// The over-long-answer counter, split by which parser
	// checkOverlongMTFrameIsRefused actually had to call (S0-close review's
	// MEDIUM-3 fix): short form has one API, but combined form has two, and
	// a dialect whose own policy is P11TagDisplay must be seen to refuse
	// through ITS OWN display-bearing parser, not the display-less one —
	// requiring only the unconditional pre-fix name would have let a
	// dispatch bug that silently reverted to one parser for both policies
	// go unnoticed here, the same way it went unnoticed in review.
	switch r.d.MTForm() {
	case cat.MTFormShort, cat.MTFormShortNoDisplay:
		requiredRefusals = append(requiredRefusals, "over-long MT answer")
	case cat.MTFormCombined:
		switch r.d.MTP11() {
		case cat.P11TagDisplay:
			requiredRefusals = append(requiredRefusals, "over-long MT answer (display-bearing)")
		default:
			requiredRefusals = append(requiredRefusals, "over-long MT answer (display-less)")
		}
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
	// The MC send and MT read domains, each counted against its OWN policy.
	// A narrow dialect with no 60m/EMG slot in its space cannot satisfy
	// either counter by having nothing to offer — that is the point: a
	// fixture declaring MCSelectsMemoryPMS or MTReadsMemoryPMS without a
	// 5xx bank or an EMG channel leaves checkMCSendDomain/checkMTReadDomain
	// vacuous over it, and this is what fails that silently.
	if r.d.MCSelects() == cat.MCSelectsMemoryPMS {
		requiredRefusals = append(requiredRefusals, "MC send refused for 60m/EMG under MCSelectsMemoryPMS")
	}
	// MCSelectsUnsupported (the FTX-1's own): checkFixedFrames' MC-read
	// arm and checkMCSendDomain's per-slot walk each have their own
	// refusal counter for it, required unconditionally (the gate refusal
	// runs whatever the slot space holds) and only when this dialect
	// actually classifies a slot to offer the send-domain walk (the same
	// non-vacuity condition every other MCSelects/PMS/tone requirement
	// below applies).
	if r.d.MCSelects() == cat.MCSelectsUnsupported {
		requiredRefusals = append(requiredRefusals, "MC read refused at the gate under MCSelectsUnsupported")
		if len(r.slots) > 0 {
			requiredRefusals = append(requiredRefusals, "MC send refused for every slot under MCSelectsUnsupported")
		}
	}
	if r.d.MTReadSlots() == cat.MTReadsMemoryPMS {
		requiredRefusals = append(requiredRefusals, "MT read refused for 60m/EMG under MTReadsMemoryPMS")
	}
	// The P8 state domain. A three-state dialect must be SEEN to refuse a
	// DCS state at each of the three places that decide it; a five-state one
	// has nothing to refuse there, and is required instead to be SEEN to
	// accept one at each of the four places that decide it. The five-state
	// arm is all absences of errors, so without these it would pass by
	// never running at all — finding L4.
	if len(r.emittable) > 0 && len(r.slots) > 0 {
		switch r.d.ToneStates() {
		case cat.ToneStatesCTCSS:
			requiredRefusals = append(requiredRefusals,
				"DCS state refused at BuildMWSet under ToneStatesCTCSS",
				"DCS state refused at the gate under ToneStatesCTCSS")
			if r.d.MTForm() == cat.MTFormCombined {
				requiredRefusals = append(requiredRefusals, "DCS state refused at BuildMTSetCombined under ToneStatesCTCSS")
			}
		case cat.ToneStatesCTCSSAndDCS:
			requiredAcceptances = append(requiredAcceptances,
				"DCS state accepted at BuildMWSet under ToneStatesCTCSSAndDCS",
				"DCS state accepted at the gate under ToneStatesCTCSSAndDCS",
				"DCS state decoded by ParseMRAnswer under ToneStatesCTCSSAndDCS")
			if r.d.MTForm() == cat.MTFormCombined {
				requiredAcceptances = append(requiredAcceptances, "DCS state accepted at BuildMTSetCombined under ToneStatesCTCSSAndDCS")
			}
		case cat.ToneStatesSix:
			requiredAcceptances = append(requiredAcceptances,
				"state accepted at BuildMWSet under ToneStatesSix",
				"state accepted at the gate under ToneStatesSix",
				"state decoded by ParseMRAnswer under ToneStatesSix")
			if r.d.MTForm() == cat.MTFormCombined {
				requiredAcceptances = append(requiredAcceptances, "state accepted at BuildMTSetCombined under ToneStatesSix")
			}
		default:
			// A domain neither arm names requires nothing, so the whole P8
			// axis would be non-vacuous by vacuity — the failure this
			// function exists to prevent (seat 2 LOW-4).
			r.t.Errorf("%s: declares tone-state domain %v, which the coverage ledger does not enumerate — its P8 legs would be required of nothing", r.name(), r.d.ToneStates())
		}
	}
	// The PMS wire form, counted against the form the dialect declares. A
	// dialect with no pairs declares no form and checkPMSSlotForm returns
	// early, so neither counter is required of it.
	if _, err := r.d.PMSSlot(1, false); err == nil {
		switch r.d.PMSForm() {
		case cat.PMSFormNumeric:
			requiredRefusals = append(requiredRefusals, "token PMS form refused under PMSFormNumeric")
		case cat.PMSFormToken:
			requiredRefusals = append(requiredRefusals, "three-digit form declined as PMS under PMSFormToken")
		case cat.PMSFormDashToken:
			requiredRefusals = append(requiredRefusals, "single-digit token PMS form refused under PMSFormDashToken")
		default:
			// As the tone-state arm above: a third form would require
			// neither counter, and checkPMSSlotForm's own sweep would be
			// witnessed by nothing (seat 2 LOW-4).
			r.t.Errorf("%s: declares PMS form %v, which the coverage ledger does not enumerate — its slot-form legs would be required of nothing", r.name(), r.d.PMSForm())
		}
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

	for _, what := range requiredAcceptances {
		if r.acceptances[what] == 0 {
			r.t.Errorf("%s: no %q was observed — a check that asserts this dialect does NOT refuse something passes by never running, so the acceptance itself has to be counted", r.name(), what)
		}
	}

	if r.exactTagRoundTrips == 0 {
		r.t.Errorf("%s: not one non-empty tag survived build -> parse intact (%d came back trimmed) — this suite offers tags with four different final bytes precisely so that a dialect's fill and pad bytes cannot spoil them all, so a dialect losing every one of them has an MT tag codec that does not round-trip", r.name(), r.trimmedTagRoundTrips)
	}
	if r.total < minConformanceFrames {
		r.t.Errorf("%s: only %d frames were built and checked in total; a dialect with a slot space, a mode table and a menu produces hundreds — this walk is not reaching the builders", r.name(), r.total)
	}

	r.t.Logf("%s (%v): %d frames checked across %d slots and %d modes; per builder: %v; refusals seen: %v; acceptances seen: %v; tag round trips: %d exact, %d trimmed",
		r.name(), r.d.MTForm(), r.total, len(r.slots), len(r.modes), r.frames, r.refusals, r.acceptances, r.exactTagRoundTrips, r.trimmedTagRoundTrips)
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
