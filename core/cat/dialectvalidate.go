// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"sort"
)

// maxEXDigits is the largest EXItem.Digits a dialect may declare.
//
// Derived, not chosen: an EX answer is "EX"(2) + address(6) + P4 + ";"(1),
// so DefaultMaxFrame - 9 is the widest P4 whose answer a FrameAccumulator
// with the default bound can still assemble. A dialect declaring more would
// describe answers its own transport can never receive whole, and
// Dialect.exAnswerMaxLen's arithmetic would run away with a large enough
// value.
//
// IT STAYS KEYED ON THE SIX-DIGIT FORM. An EXAddressPair dialect's own
// overhead is 7, not 9, so this ceiling is CONSERVATIVE for it by two
// bytes: it can refuse a P4 that radio's transport could in fact have
// assembled, and can never admit one it could not. Widening it per form
// would move a bound that internal/extable.MaxDigitsCeiling mirrors as a
// single number (exdigits_ceiling_test.go pins the two equal), for two
// bytes of headroom no chart is anywhere near.
const maxEXDigits = DefaultMaxFrame - 9 // 247

// maxMTTagBytes is the largest MTPolicy.TagMaxBytes a dialect may declare.
//
// A RESOURCE bound, not a protocol fact — no classic-family radio documents
// a tag beyond 12. It exists because TagMaxBytes bounds the OUTBOUND WRITE
// GATE: without a ceiling, one mistyped digit authorises the gate to admit
// a pathologically long MT frame. 64 keeps the resulting short frame
// (2+3+1+64+1 = 71 bytes) comfortably inside DefaultMaxFrame while leaving
// five times the widest documented tag.
const maxMTTagBytes = 64

// maxSlotDecimal is the largest value a 3-digit numeric slot can express.
const maxSlotDecimal = 999

// maxEXComponent is the largest value an EXAddress component may hold
// UNDER EXAddressTriple AND EXAddressPair, and — under EXAddressSingle —
// the largest P2 and P3 may hold, which V12 separately requires to be zero.
//
// wireEXAddress renders each of those components with %02d, which is a
// MINIMUM width, not an exact one: a component of 100 renders three digits
// and produces a seven-byte address in a field the grammar fixes at six (or
// a five-byte one in the four-digit field). The resulting frame is rejected
// by the dialect's own gate and can never be reconstructed by its own
// parser, because ParseEXAddress consumes exactly the declared width.
//
// THE BOUND IS FORM-DEPENDENT, and this constant is only two thirds of it.
// EXAddressSingle renders P1 with %03d into a three-digit field, so that
// component's ceiling is maxEXComponentSingleP1 below. Both are applied by
// V8 (validateEXItems), which selects between them on cfg.EXAddressForm.
//
// The type is NOT the bound. EXAddress's components were uint8 until the
// FT-991A seam, and that paragraph used to end "uint8 alone therefore does
// not constrain this enough" — true, but it invited the reading that the
// type constrained it at all. It is uint16 now precisely so that the STATED
// rule is the operative one: a P1 of 300 is representable, legal under
// Single and refused under Triple by this constant, which is a disagreement
// no uint8 field could express. TestValidateEXItems_ComponentBoundIsForm-
// Dependent pins it.
const maxEXComponent = 99

// maxEXComponentSingleP1 is the largest P1 an EXAddressSingle dialect may
// hold: the three-digit field's own capacity, 999.
//
// It is a SEPARATE constant rather than a number computed from a width,
// because it is read beside maxEXComponent by the same rule and the two must
// be legible together. wireEXAddress renders %03d under this form — again a
// MINIMUM width — so a P1 of 1000 would produce a four-digit address in a
// field the FT-991A's grammar fixes at three, built and gate-approved and
// unparseable by this dialect's own ParseEXAddress.
//
// The FT-991A's chart stops at 153. The bound is the FIELD's, not the
// chart's: membership is what refuses 154, and this rule exists for the
// address the field could never carry at all.
const maxEXComponentSingleP1 = 999

// clarFieldMaxHz is the largest magnitude the 4-digit clarifier field can
// carry, whatever a family's step size.
const clarFieldMaxHz = 9999

// validateDialectConfig runs every rule and returns the first failure.
//
// Errors name the FIELD and the OFFENDING VALUE. That is a contract the
// tests assert on, not a nicety: a validator that returns a generic error
// from the wrong branch passes a test that only checks for non-nil, and
// this package has shipped exactly that kind of silently-correct-looking
// check before.
func validateDialectConfig(cfg DialectConfig) error {
	for _, rule := range []func(DialectConfig) error{
		validateCATID,         // V1
		validateModeNames,     // V2
		validatePMSPairs,      // V3
		validatePMSForm,       // V15 — see its own comment for why it runs HERE
		validateSpecialWires,  // V4
		validateMemoryRange,   // V5
		validateSixtyRange,    // V6
		validateShadowing,     // V7
		validateEXItems,       // V8
		validateMTPolicy,      // V9
		validateClarifier,     // V10
		validateMWWriteKind,   // V11
		validateEXAddressForm, // V12
		validateMCSelects,     // V13
		validateMemoryP5,      // V14
		validateToneStates,    // V16
	} {
		if err := rule(cfg); err != nil {
			return err
		}
	}
	return nil
}

// validateCATID is V1. The ID answer frame is "ID" + 4 bytes + ";", so a
// CATID of any other width could never match a real answer — connect-time
// identification would simply never succeed, silently.
func validateCATID(cfg DialectConfig) error {
	if len(cfg.CATID) != 4 {
		return fmt.Errorf("cat: CATID %q is %d bytes, want exactly 4 (the ID answer is \"ID\"+4+\";\")", cfg.CATID, len(cfg.CATID))
	}
	if !validWireString(cfg.CATID) {
		return fmt.Errorf("cat: CATID %q contains a byte outside printable ASCII 0x20-0x7E excluding ';'", cfg.CATID)
	}
	return nil
}

// validateModeNames is V2, which has four clauses.
//
// The KEY clause is a write-gate rule: BuildMWSet writes the key byte
// directly into the frame's P6 field, and the gate validates through the
// same receiver, so a key outside the wire domain yields a gate-approved
// frame carrying it.
//
// The NAME clauses make the display-name mapping invertible, which is what
// Dialect.ModeByName depends on: with two modes sharing a name, one
// silently wins and a channel is written with the wrong mode nibble.
func validateModeNames(cfg DialectConfig) error {
	if len(cfg.ModeNames) == 0 {
		return fmt.Errorf("cat: ModeNames is empty, want at least one mode")
	}
	// Sorted iteration so the reported failure is the same one every run:
	// map order is randomised, and a validator that reports a different
	// offender per run makes a failing test look flaky.
	keys := make([]int, 0, len(cfg.ModeNames))
	for m := range cfg.ModeNames {
		keys = append(keys, int(m))
	}
	sort.Ints(keys)

	seen := make(map[string]Mode, len(cfg.ModeNames))
	for _, k := range keys {
		m := Mode(k)
		name := cfg.ModeNames[m]
		if !validWireByte(byte(m)) {
			return fmt.Errorf("cat: ModeNames key %#02x is outside printable ASCII 0x20-0x7E excluding ';' — it would be written into an MW frame's P6 field and admitted by this dialect's own gate", byte(m))
		}
		if name == "" {
			return fmt.Errorf("cat: ModeNames[%#02x] is empty, want a display name", byte(m))
		}
		if prev, dup := seen[name]; dup {
			return fmt.Errorf("cat: ModeNames has duplicate name %q for modes %#02x and %#02x — display names must be invertible (see Dialect.ModeByName)", name, byte(prev), byte(m))
		}
		seen[name] = m
	}
	return nil
}

// validatePMSPairs is V3, and its ceiling is FORM-AWARE.
//
// The 0..9 bound is the TOKEN form's, and so is its reason: the pair number
// sits between 'P' and 'L'/'U' as a single ASCII digit, so a tenth pair
// builds a wire form this dialect's own ParseSlot rejects. That reason is
// untrue under PMSFormNumeric, where the pair number never reaches the wire
// at all and the slots are ordinary decimal channel numbers — the operative
// ceiling there is V15's (the range must end at or below 999), derived from
// the wire form the dialect actually builds. Dialect.pmsCap makes the same
// split, so a numeric dialect declaring more than nine pairs is BUILT with
// them rather than silently clamped.
//
// The token sentence is unchanged, byte for byte, and the test that pins it
// is TestV3_PairBoundIsFormAware. An omitted form takes the token bound
// here and is then refused by V15 at the next position, so no config
// escapes a ceiling.
func validatePMSPairs(cfg DialectConfig) error {
	n := cfg.Slots.PMSPairs
	if n < 0 {
		return fmt.Errorf("cat: Slots.PMSPairs is %d, want >= 0 — a negative pair count describes no slot either form could build", n)
	}
	if cfg.Slots.PMSForm != PMSFormNumeric && n > 9 {
		return fmt.Errorf("cat: Slots.PMSPairs is %d, want 0..9 — the wire form's pair number is a single ASCII digit, so a larger value builds forms this dialect's own ParseSlot rejects", n)
	}
	return nil
}

// validatePMSForm is V15: the PMS wire form must be declared, never
// inferred, and the numeric form's base must describe a range a 3-digit
// slot can express.
//
// IT RUNS AT RULE POSITION 4, immediately after V3 and before V5/V6/V7, and
// the position is load-bearing. validateDialectConfig returns the FIRST
// error, and both V6 and V7 consult PMSForm and PMSNumericLo: a config with
// PMSPairs 9 and PMSForm omitted would otherwise be diagnosed by V6 as
// "memory range 1..99 overlaps PMS numeric range 0..17" — a range nobody
// configured — instead of by this rule's honest "declare a form". The
// package already documents the mirror hazard at renderEXAddressForV8,
// where V8 needs a special renderer because it runs BEFORE V12. Inserting
// here renumbers no V-LABEL: the V-numbers are comment labels, not indices,
// which is why this rule is V15 in a slice it enters fourth. It does
// renumber POSITIONS, and one comment in this file states one: V8 moved
// from position 8 to 9, corrected in validateEXItems' own doc (Stage 0
// close review, both seats, MEDIUM-2). A comment that states a position
// rather than a label has to move with the slice.
//
// An omitted config semantic is REFUSED, never defaulted, and the cost of a
// default is a WRITE cost — see PMSSlotForm's own doc comment.
func validatePMSForm(cfg DialectConfig) error {
	s := cfg.Slots
	switch s.PMSForm {
	case PMSFormToken:
		if s.PMSNumericLo != 0 {
			return fmt.Errorf("cat: Slots.PMSNumericLo is %d under %v, want exactly 0 — the token form's pairs carry no decimal numbering, so a base here describes nothing this dialect builds", s.PMSNumericLo, s.PMSForm)
		}
	case PMSFormNumeric:
		if s.PMSNumericLo < 1 {
			return fmt.Errorf("cat: Slots.PMSNumericLo is %d under %v, want >= 1 — pair 1's lower slot is a decimal channel number, and 0 collides with the \"000\" none form every registered family declares", s.PMSNumericLo, s.PMSForm)
		}
		if hi := s.PMSNumericLo + 2*s.PMSPairs - 1; hi > maxSlotDecimal {
			return fmt.Errorf("cat: Slots.PMSNumericLo %d with PMSPairs %d reaches %d, want <= %d — a slot wire form is 3 digits, so the pairs above that could never be built or parsed", s.PMSNumericLo, s.PMSPairs, hi, maxSlotDecimal)
		}
		// The same dead configuration the default arm refuses by name, seen
		// from inside the numeric form: a base with no pairs numbers nothing,
		// and pmsCap() returning 0 makes every PMS route error anyway. It was
		// ACCEPTED here whilst being refused there, which is the asymmetry
		// the adversarial review recorded as finding L1.
		if s.PMSPairs < 1 {
			return fmt.Errorf("cat: Slots.PMSPairs is %d under %v with PMSNumericLo %d — a numeric base with nothing to number is dead configuration", s.PMSPairs, s.PMSForm, s.PMSNumericLo)
		}
	default:
		// A value that is not the ZERO one is no member of this type at all,
		// and it is refused UNCONDITIONALLY — the rule V14 (validateMemoryP5)
		// and V16 (validateToneStates) apply to their own enums. Only
		// PMSSlotForm(0) can be a legitimate omission, and only for a dialect
		// that declares no PMS pairs and no numeric base; a garbage value is
		// a transcription error whether or not there are pairs beside it, and
		// this was the one place in the lane where an undeclared enum value
		// survived construction (finding L1).
		if s.PMSForm != PMSSlotForm(0) {
			return fmt.Errorf("cat: Slots.PMSForm is %v, which is not a declared member — declare PMSFormToken or PMSFormNumeric (an omitted config semantic is refused, never defaulted, and an undeclared one all the more so)", s.PMSForm)
		}
		if s.PMSPairs > 0 {
			return fmt.Errorf("cat: Slots.PMSForm is %v with PMSPairs %d — declare PMSFormToken or PMSFormNumeric explicitly (an omitted config semantic is refused, never defaulted; the two forms put different bytes on the wire, and writableSlot admits either)", s.PMSForm, s.PMSPairs)
		}
		if s.PMSNumericLo != 0 {
			return fmt.Errorf("cat: Slots.PMSNumericLo is %d with no PMSForm declared and no PMS pairs, want exactly 0 — a numeric base with nothing to number is dead configuration", s.PMSNumericLo)
		}
	}
	return nil
}

// validateSpecialWires is V4.
//
// classifySlot matches these by EXACT STRING before applying any character
// grammar, so a non-printable byte here reaches the wire: "\x00AB" as an
// emergency form produces a side-effecting MC frame this dialect's gate
// admits. A length other than 3 is dead configuration — classifySlot
// returns slotKindInvalid for every wire form that is not 3 bytes — which
// fails silently rather than loudly.
func validateSpecialWires(cfg DialectConfig) error {
	for _, w := range []struct {
		field, value string
	}{
		{"Slots.EmergencyWire", cfg.Slots.EmergencyWire},
		{"Slots.NoneWire", cfg.Slots.NoneWire},
	} {
		if w.value == "" {
			continue // absent, which is legitimate for both
		}
		if len(w.value) != 3 {
			return fmt.Errorf("cat: %s is %q (%d bytes), want exactly 3 or \"\" — classifySlot rejects every other length, so this would never match", w.field, w.value, len(w.value))
		}
		if !validWireString(w.value) {
			return fmt.Errorf("cat: %s is %q, which contains a byte outside printable ASCII 0x20-0x7E excluding ';'", w.field, w.value)
		}
	}
	return nil
}

// validateMemoryRange is V5.
//
// MemoryLo == 0 is PERMITTED. A radio numbering its channels from 000 is
// representable and one of this package's own peer fixtures depends on it.
// The hazard that creates — a "000" none-form shadowing channel 000 — is
// V7's, because it is a collision between two fields rather than a bad
// range. An earlier draft of this rule required MemoryLo >= 1 and would
// have rejected that fixture outright.
func validateMemoryRange(cfg DialectConfig) error {
	return validateSlotRange("Slots.Memory", cfg.Slots.MemoryLo, cfg.Slots.MemoryHi)
}

// validateSixtyRange is V6: the same range rule, plus non-overlap — now
// over THREE numeric intervals rather than two.
//
// classifySlot tests the memory range BEFORE the 60m range and both before
// the numeric PMS range, so an overlap is not ambiguous at runtime — the
// earlier arm simply wins, and every colliding slot is silently
// misclassified.
//
// THE THIRD INTERVAL IS PMSFormNumeric'S, and it is what makes slot.go's
// "the four static kinds are not a second opinion" invariant true under
// that form: PMSSlot hard-codes kind: slotKindPMS, so the numeric interval
// must be disjoint from the other two or a slot the constructor calls PMS
// is a memory channel to the same dialect's own classifier. Under
// PMSFormToken there is no interval and these clauses are inert.
// TestV15_PMSFormRefusals' two overlap cases pin them.
func validateSixtyRange(cfg DialectConfig) error {
	if err := validateSlotRange("Slots.Sixty", cfg.Slots.SixtyLo, cfg.Slots.SixtyHi); err != nil {
		return err
	}
	s := cfg.Slots
	if s.MemoryHi > 0 && s.SixtyHi > 0 && s.MemoryLo <= s.SixtyHi && s.SixtyLo <= s.MemoryHi {
		return fmt.Errorf("cat: memory range %d..%d overlaps 60m range %d..%d — classifySlot checks memory first, so every slot in the overlap would be classified as an ordinary channel", s.MemoryLo, s.MemoryHi, s.SixtyLo, s.SixtyHi)
	}
	// V15 has already run (rule position 4), so under the numeric form the
	// base is >= 1 and the top is <= 999 before this arithmetic happens.
	if lo, hi, ok := numericPMSInterval(s); ok {
		if s.MemoryHi > 0 && s.MemoryLo <= hi && lo <= s.MemoryHi {
			return fmt.Errorf("cat: memory range %d..%d overlaps PMS numeric range %d..%d — classifySlot checks memory first, so every PMS slot in the overlap would be classified as an ordinary channel", s.MemoryLo, s.MemoryHi, lo, hi)
		}
		if s.SixtyHi > 0 && s.SixtyLo <= hi && lo <= s.SixtyHi {
			return fmt.Errorf("cat: 60m range %d..%d overlaps PMS numeric range %d..%d — classifySlot checks 60m first, so every PMS slot in the overlap would be classified as a 60m channel", s.SixtyLo, s.SixtyHi, lo, hi)
		}
	}
	return nil
}

// numericPMSInterval returns the inclusive decimal range s's PMS pairs
// occupy under PMSFormNumeric, and whether it has one at all.
//
// It is the CONFIG-side twin of Dialect.numericPMSRange, which the
// classifier consults; the two compute the same arithmetic from the same
// two fields, because a validator runs before any Dialect exists. Nothing
// stores the range's top: it is derived from PMSNumericLo and PMSPairs in
// both places, so there is no second field for either to drift from.
func numericPMSInterval(s SlotSpace) (lo, hi int, ok bool) {
	if s.PMSForm != PMSFormNumeric || s.PMSPairs <= 0 {
		return 0, 0, false
	}
	return s.PMSNumericLo, s.PMSNumericLo + 2*s.PMSPairs - 1, true
}

// validateSlotRange is the shared range rule for V5 and V6: absent is
// exactly (0,0); otherwise 0 <= lo <= hi <= 999.
//
// Requiring exactly (0,0) for absence rejects dead configurations such as
// (99, 0), which reads as "present" to a human and "absent" to
// classifySlot, whose activation test is hi > 0.
func validateSlotRange(field string, lo, hi int) error {
	if lo == 0 && hi == 0 {
		return nil // absent
	}
	if hi <= 0 {
		return fmt.Errorf("cat: %sHi is %d with %sLo %d — express an absent range as exactly (0, 0); classifySlot activates on Hi > 0, so this reads as absent while looking present", field, hi, field, lo)
	}
	if lo < 0 || lo > hi {
		return fmt.Errorf("cat: %s range is %d..%d, want 0 <= Lo <= Hi", field, lo, hi)
	}
	if hi > maxSlotDecimal {
		return fmt.Errorf("cat: %sHi is %d, want <= %d — a slot wire form is 3 digits", field, hi, maxSlotDecimal)
	}
	return nil
}

// validateShadowing is V7, and it is the rule most likely to catch a real
// second radio's transcription.
//
// classifySlot tests noneWire and emergencyWire FIRST, before any numeric
// or PMS classification. A special form that also falls inside an active
// numeric range, or that spells a PMS form this dialect can build, is
// therefore consumed by the special branch and the other meaning is lost —
// with no error anywhere.
func validateShadowing(cfg DialectConfig) error {
	s := cfg.Slots
	if s.NoneWire != "" && s.NoneWire == s.EmergencyWire {
		return fmt.Errorf("cat: Slots.NoneWire and Slots.EmergencyWire are both %q — classifySlot tests NoneWire first, so the emergency meaning would be unreachable", s.NoneWire)
	}
	for _, w := range []struct {
		field, value string
	}{
		{"Slots.EmergencyWire", s.EmergencyWire},
		{"Slots.NoneWire", s.NoneWire},
	} {
		if w.value == "" {
			continue
		}
		if n, ok := decimalWire(w.value); ok {
			if s.MemoryHi > 0 && n >= s.MemoryLo && n <= s.MemoryHi {
				return fmt.Errorf("cat: %s is %q, which falls inside the memory range %d..%d — classifySlot tests it first, so memory channel %d would be unreachable", w.field, w.value, s.MemoryLo, s.MemoryHi, n)
			}
			if s.SixtyHi > 0 && n >= s.SixtyLo && n <= s.SixtyHi {
				return fmt.Errorf("cat: %s is %q, which falls inside the 60m range %d..%d — classifySlot tests it first, so 60m slot %d would be unreachable", w.field, w.value, s.SixtyLo, s.SixtyHi, n)
			}
		}
		if pmsWireInRange(w.value, s) {
			return fmt.Errorf("cat: %s is %q, which is also a PMS form this dialect can build (PMSPairs %d) — PMSSlot would return a wire form classifySlot reports as something else", w.field, w.value, s.PMSPairs)
		}
	}
	return nil
}

// decimalWire parses a 3-byte all-digit wire form. The bool reports
// whether it was all digits, mirroring classifySlot's own test rather than
// re-deriving it differently.
func decimalWire(wire string) (int, bool) {
	if len(wire) != 3 {
		return 0, false
	}
	for i := 0; i < len(wire); i++ {
		if wire[i] < '0' || wire[i] > '9' {
			return 0, false
		}
	}
	return int(wire[0]-'0')*100 + int(wire[1]-'0')*10 + int(wire[2]-'0'), true
}

// pmsWireInRange reports whether wire is a PMS form a dialect with slot
// space s would build and classify.
//
// IT IS FORM-AWARE, and it has to be. Form-blind it asked one question —
// "does this spell P<1..pairs><L|U>?" — which under PMSFormNumeric is the
// wrong question in both directions: it would refuse a NoneWire of "P1L"
// for a collision that cannot happen (that dialect builds no token form at
// all) and would NOT refuse one of "100", which really is the wire form its
// own PMSSlot builds for pair 1. V7 exists to catch exactly that shadowing.
func pmsWireInRange(wire string, s SlotSpace) bool {
	if s.PMSPairs <= 0 || len(wire) != 3 {
		return false
	}
	if lo, hi, ok := numericPMSInterval(s); ok {
		n, allDigits := decimalWire(wire)
		return allDigits && n >= lo && n <= hi
	}
	return wire[0] == 'P' &&
		wire[1] >= '1' && wire[1] <= byte('0'+s.PMSPairs) &&
		(wire[2] == 'L' || wire[2] == 'U')
}

// validateEXItems is V8.
//
// Its three address renders go through wireEXAddress with the config's OWN
// form, not through EXAddress's %v: this rule runs before any Dialect
// exists, so cfg.EXAddressForm is the only form in scope, and rendering an
// address here as the debug String() would have moved three shipped error
// sentences. TestValidateEXItems_TripleErrorTextIsByteIdentical pins all
// three against their pre-seam spelling.
//
// V8 runs at rule position 9, four places before V12
// (validateEXAddressForm) refuses a zero form — so a config that omits
// EXAddressForm AND fails V8 reaches this renderer first, with
// wireEXAddress(0, addr) returning "". renderEXAddressForV8 falls back to
// the debug String() form in that case, so the message still names the
// address rather than rendering an empty string.
// TestValidateEXItems_ZeroFormFallsBackToDebugForm pins this. The rule
// ORDER is untouched — V12 still runs fourth after V8 — this only changes
// what V8 renders when asked to render through a form it cannot.
func validateEXItems(cfg DialectConfig) error {
	seen := make(map[EXAddress]int, len(cfg.EXItems))
	for i, it := range cfg.EXItems {
		for ci, c := range []struct {
			name string
			v    uint16
		}{{"P1", it.Addr.P1}, {"P2", it.Addr.P2}, {"P3", it.Addr.P3}} {
			// P1 under EXAddressSingle is the ONE component with a wider
			// domain: its field is three digits, so its ceiling is
			// maxEXComponentSingleP1 and its refusal is its own sentence.
			// The Triple/Pair sentence below is SHIPPED TEXT and must not
			// move by a byte, which is why the two are separate literals
			// rather than one composed from whichever number is in force.
			if ci == 0 && cfg.EXAddressForm == EXAddressSingle {
				if int(c.v) > maxEXComponentSingleP1 {
					return fmt.Errorf("cat: EXItems[%d].Addr.P1 is %d, want <= %d under %v — wireEXAddress renders %%03d under this form, a MINIMUM width, so a larger P1 overruns the three-digit address field this dialect's own ParseEXAddress reads back", i, c.v, maxEXComponentSingleP1, cfg.EXAddressForm)
				}
				continue
			}
			if int(c.v) > maxEXComponent {
				return fmt.Errorf("cat: EXItems[%d].Addr.%s is %d, want <= %d — wireEXAddress renders %%02d, a MINIMUM width, so a larger component overruns the fixed-width address field this dialect's own ParseEXAddress reads back", i, c.name, c.v, maxEXComponent)
			}
		}
		if prev, dup := seen[it.Addr]; dup {
			return fmt.Errorf("cat: EXItems[%d] repeats address %s, already at index %d", i, renderEXAddressForV8(cfg.EXAddressForm, it.Addr), prev)
		}
		seen[it.Addr] = i
		if it.Digits < 1 {
			return fmt.Errorf("cat: EXItems[%d] (%s) has Digits %d, want >= 1", i, renderEXAddressForV8(cfg.EXAddressForm, it.Addr), it.Digits)
		}
		if it.Digits > maxEXDigits {
			return fmt.Errorf("cat: EXItems[%d] (%s) has Digits %d, want <= %d — a wider P4 describes an answer frame longer than DefaultMaxFrame (%d), which this dialect's own transport could never assemble", i, renderEXAddressForV8(cfg.EXAddressForm, it.Addr), it.Digits, maxEXDigits, DefaultMaxFrame)
		}
	}
	return nil
}

// renderEXAddressForV8 renders addr through form for a V8 error message,
// falling back to addr's debug String() form when form is the zero value
// (wireEXAddress(0, addr) is ""). V8 runs before V12, so a config that
// omits EXAddressForm can still reach here; this keeps its refusal message
// naming the address rather than rendering an empty string.
// TestValidateEXItems_ZeroFormFallsBackToDebugForm pins it.
func renderEXAddressForV8(form EXAddressForm, addr EXAddress) string {
	if form == EXAddressForm(0) {
		return addr.String()
	}
	return wireEXAddress(form, addr)
}

// validateMTPolicy is V9: the TagMaxBytes ceiling, then per-form field
// ownership.
//
// There is deliberately NO runtime frame-ceiling branch here, though the
// spec asks V9 to prove each form's derived maximum fits DefaultMaxFrame.
// The TagMaxBytes cap below makes both maxima compile-time facts — the
// short form reaches at most mtAnswerMinLen+64 = 71 bytes and the combined
// form at most 29+64 = 93, against a DefaultMaxFrame of 256 — so no config
// this validator admits can reach such a branch, and a branch no input can
// take is dead code pretending to be a proof. The relationship is asserted
// instead as a constants invariant, in
// TestMTFrameCeilings_FitTransportFrame.
func validateMTPolicy(cfg DialectConfig) error {
	if n := cfg.MT.TagMaxBytes; n < 1 || n > maxMTTagBytes {
		return fmt.Errorf("cat: MT.TagMaxBytes is %d, want 1..%d — it bounds the outbound write gate, so an unbounded value would authorise a pathologically long MT frame", n, maxMTTagBytes)
	}
	// ReadSlots, BEFORE the form switch: the MT read request carries neither
	// a record nor a tag, so it is the one part of this command that is the
	// same shape under both forms, and a config omitting its domain must be
	// refused whichever form it declares. An omitted config semantic is
	// REFUSED, never defaulted — and defaulting this one to the wide reading
	// would have BuildMTRead emit, and this dialect's own gate admit, a read
	// of a bank the radio's MT block never lists.
	switch cfg.MT.ReadSlots {
	case MTReadsReadable, MTReadsMemoryPMS:
	default:
		return fmt.Errorf("cat: MT.ReadSlots is %v, which is not a policy — declare MTReadsReadable or MTReadsMemoryPMS explicitly (MT's read domain is not always MR's)", cfg.MT.ReadSlots)
	}
	switch cfg.MT.Form {
	case MTFormShort:
		// The pre-existing short-form requirements, verbatim: ClearTagByte
		// must be a valid wire byte (it is emitted into every cleared
		// tag), and PadByte keeps its 0-or-valid rule. Only TagFill is
		// new here, as an ownership refusal.
		if !validWireByte(cfg.MT.ClearTagByte) {
			return fmt.Errorf("cat: MT.ClearTagByte is %#02x, which is outside printable ASCII 0x20-0x7E excluding ';' — it is emitted into every cleared tag", cfg.MT.ClearTagByte)
		}
		if cfg.MT.PadByte != 0 && !validWireByte(cfg.MT.PadByte) {
			return fmt.Errorf("cat: MT.PadByte is %#02x, want 0 (no padding) or a byte inside printable ASCII 0x20-0x7E excluding ';'", cfg.MT.PadByte)
		}
		if cfg.MT.TagFill != 0 {
			return fmt.Errorf("cat: MT.TagFill %#02x is set under MTFormShort — TagFill is combined-form data and an inapplicable field must be explicitly zero", cfg.MT.TagFill)
		}
		if cfg.MT.P11 != 0 {
			return fmt.Errorf("cat: MT.P11 %v is set under MTFormShort — P11 is the COMBINED record's byte 28, and the short form's display flag is already a parameter of BuildMTSet; an inapplicable field must be explicitly zero", cfg.MT.P11)
		}
	case MTFormCombined:
		if cfg.MT.ClearTagByte != 0 {
			return fmt.Errorf("cat: MT.ClearTagByte %#02x is set under MTFormCombined — no distinct clear encoding is documented for the combined form; an empty tag is the all-TagFill field", cfg.MT.ClearTagByte)
		}
		if cfg.MT.PadByte != 0 {
			return fmt.Errorf("cat: MT.PadByte %#02x is set under MTFormCombined — answer trimming is TagFill's job in this form", cfg.MT.PadByte)
		}
		if !validWireByte(cfg.MT.TagFill) {
			return fmt.Errorf("cat: MT.TagFill is %#02x under MTFormCombined, want printable ASCII 0x20-0x7E excluding ';' — it fills every outbound tag field, and zero would silently emit NUL", cfg.MT.TagFill)
		}
		switch cfg.MT.P11 {
		case P11Fixed, P11TagDisplay:
		default:
			return fmt.Errorf("cat: MT.P11 is %v, which is not a policy — declare P11Fixed or P11TagDisplay explicitly (byte 28 of the combined record is a printed-fixed '0' on some radios and a live TAG flag on others, and a live flag is never defaulted)", cfg.MT.P11)
		}
	default:
		return fmt.Errorf("cat: MT.Form %v must be set explicitly — the zero value is not a form (an omitted form must refuse, not default)", cfg.MT.Form)
	}
	return nil
}

// validateClarifier is V10.
func validateClarifier(cfg DialectConfig) error {
	c := cfg.Clarifier
	if c.StepHz < 1 {
		return fmt.Errorf("cat: Clarifier.StepHz is %d, want >= 1 — validClarHz takes a modulo by it", c.StepHz)
	}
	// An upper bound as well as a lower one (milestone review, finding 1).
	// A step wider than the whole 4-digit field is meaningless, and without
	// this an enormous StepHz passed every clause here — MaxAbsHz 0 is >= 0,
	// is <= 9999, and 0 % anything is 0 — and then panicked validClarHz,
	// which narrowed it to int16 and divided by the resulting zero. That
	// narrowing is gone too, but a policy this validator cannot justify
	// should not reach a dialect in the first place.
	if c.StepHz > clarFieldMaxHz {
		return fmt.Errorf("cat: Clarifier.StepHz is %d, want <= %d — a step wider than the 4-digit clarifier field cannot describe any legal value", c.StepHz, clarFieldMaxHz)
	}
	if c.MaxAbsHz < 0 {
		return fmt.Errorf("cat: Clarifier.MaxAbsHz is %d, want >= 0", c.MaxAbsHz)
	}
	if c.MaxAbsHz > clarFieldMaxHz {
		return fmt.Errorf("cat: Clarifier.MaxAbsHz is %d, want <= %d — the clarifier field is 4 digits", c.MaxAbsHz, clarFieldMaxHz)
	}
	if c.MaxAbsHz%c.StepHz != 0 {
		return fmt.Errorf("cat: Clarifier.MaxAbsHz %d is not a multiple of StepHz %d — the range's own endpoint would be rejected by this dialect's step rule", c.MaxAbsHz, c.StepHz)
	}
	return nil
}

// validateMWWriteKind is V11. An unlisted byte would be written into P7 and
// admitted by this dialect's own gate, since the builder and the gate share
// this validator.
func validateMWWriteKind(cfg DialectConfig) error {
	if !validMWWriteKindByte(cfg.MWWriteKind) {
		return fmt.Errorf("cat: MWWriteKind is %q, which is not a P7 value a builder may EMIT — KindUnset ('4') is the documented \"-\" placeholder that parsers must accept and builders must never write", cfg.MWWriteKind)
	}
	return nil
}

// validMWWriteKindByte reports whether b is a P7 value a BUILDER may emit.
//
// It is deliberately NOT validKindByte. That predicate is the READ-side
// domain: it accepts KindUnset ('4') because ParseMRAnswer must, since the
// reference documents '4' as a "-" placeholder a radio can send. A builder
// must never write it — memdata.go says so at KindUnset's own declaration.
//
// V11 delegated to validKindByte until the M9c-0 milestone review (finding
// 2), so a dialect could declare MWWriteKind: KindUnset and its builder
// would then emit P7 '4' with its own gate admitting the frame. Reproduced:
// "MW005007100000+000000240000;", built and gate-approved. Reusing a
// read-side domain for a write-side decision is the whole of the bug, and
// separating the two predicates is the whole of the fix.
func validMWWriteKindByte(b byte) bool {
	switch b {
	case KindVFO, KindMemory, KindMemTune, KindQMB, KindPMS:
		return true
	default: // KindUnset and anything undocumented
		return false
	}
}

// validateEXAddressForm is V12: the EX address field's width, and the P3
// rule the narrow form implies.
//
// The zero value is REFUSED rather than defaulted, for the M9c-1 reason and
// with a concrete cost behind it: defaulting to EXAddressTriple would give
// a four-digit radio a nine-byte EX read where its grammar prints seven —
// built, admitted by this dialect's own gate, and sent — and would make
// every answer that radio sent back unparseable. The rule fires even for an
// empty EXItems, because the width also sizes the read frame the gate
// measures, not only the addresses in the inventory.
//
// The Pair clause is the other half of wireEXAddress's four-digit render:
// that render drops P3, so a member carrying a non-zero one would lose it
// from every frame silently. The Single clause is the same rule one
// component further down — that render drops P2 as well as P3 — and it is
// the half of the FT-991A's chart shape this validator owns: the printed
// menu number IS the whole address, so any other component names something
// no frame can carry. The refusal names the offending index AND the address
// as the frame would have carried it, through the same renderer, so a
// three-digit chart's inventory does not have to be searched by hand: the
// DOMAIN a Single form's bound must cover is 0..999, of which the FT-991A's
// own chart populates 153 rows.
func validateEXAddressForm(cfg DialectConfig) error {
	switch cfg.EXAddressForm {
	case EXAddressTriple:
		return nil
	case EXAddressPair:
		for i, it := range cfg.EXItems {
			if it.Addr.P3 != 0 {
				return fmt.Errorf("cat: EXItems[%d] (%s) has P3 %d under %v — the four-digit field renders P1 and P2 only, so a non-zero P3 would be dropped from every frame this dialect builds", i, wireEXAddress(cfg.EXAddressForm, it.Addr), it.Addr.P3, cfg.EXAddressForm)
			}
		}
		return nil
	case EXAddressSingle:
		for i, it := range cfg.EXItems {
			if it.Addr.P2 != 0 || it.Addr.P3 != 0 {
				return fmt.Errorf("cat: EXItems[%d] (%s) has P2 %d and P3 %d under %v — the three-digit field renders P1 only, so a non-zero P2 or P3 would be dropped from every frame this dialect builds", i, wireEXAddress(cfg.EXAddressForm, it.Addr), it.Addr.P2, it.Addr.P3, cfg.EXAddressForm)
			}
		}
		return nil
	default:
		return fmt.Errorf("cat: EXAddressForm %v must be set explicitly — the zero value is not a form (an omitted form must refuse, not default)", cfg.EXAddressForm)
	}
}

// validateMCSelects is V13: the MC command's SEND-side slot domain must be
// declared, never inferred.
//
// An omitted config semantic is REFUSED, not defaulted. The two policies
// differ by the 60m and EMG banks, and an MC Set is SIDE-EFFECTING — it
// recalls the channel on the radio — so a family whose MC legend prints
// memory and PMS only, silently given the wider domain, would have frames
// its own manual never describes built AND admitted by its own gate (this
// rule's field reaches AllowedCommand through validMCCommand).
func validateMCSelects(cfg DialectConfig) error {
	switch cfg.Slots.MCSelects {
	case MCSelectsAll, MCSelectsMemoryPMS:
		return nil
	default:
		return fmt.Errorf("cat: Slots.MCSelects is %v, which is not a policy — declare MCSelectsAll or MCSelectsMemoryPMS explicitly (an omitted config semantic is refused, never defaulted; MC's send domain is not always MR's read domain)", cfg.Slots.MCSelects)
	}
}

// validateMemoryP5 is V14: byte 21 of the shared memory field block must be
// declared, never inferred.
//
// An omitted config semantic is REFUSED, not defaulted. Defaulting to
// P5TxClar would have this codec emit a '1' into a byte a radio's own manual
// prints "(Fixed)" — a frame that manual never describes, built and admitted
// by this dialect's own gate, since this field reaches AllowedCommand's MW
// and combined-MT checks through parseMemoryFields, which decodes byte 21
// under this same policy before validateMWFields/validateCombinedMTFields
// ever run. Defaulting to P5Fixed would silently drop a real TX-clarifier
// flag on the floor. Neither default is safe, which is exactly when a field
// must be declared.
func validateMemoryP5(cfg DialectConfig) error {
	switch cfg.MemoryP5 {
	case P5TxClar, P5Fixed:
		return nil
	default:
		return fmt.Errorf("cat: MemoryP5 is %v, which is not a policy — declare P5TxClar or P5Fixed explicitly (byte 21 of the memory block is the TX clarifier flag on some radios and a printed-fixed '0' on others)", cfg.MemoryP5)
	}
}

// validateToneStates is V16: the P8 state domain must be declared, never
// inferred.
//
// An omitted config semantic is REFUSED, not defaulted, and here neither
// default is safe. Defaulting to ToneStatesCTCSS would silently drop a real
// DCS state on the floor for a radio whose legend prints five; defaulting
// to ToneStatesCTCSSAndDCS would authorise this codec to emit P8 '3' or '4'
// into an MW or combined-MT frame for the four registered siblings, whose
// manuals print 0/1/2 only — built AND admitted by their own gates, since
// this field reaches AllowedCommand through validateMWFields and
// validateCombinedMTFields as well as through parseMemoryFields.
//
// It runs LAST, at rule position 16. Nothing else consults ToneStates, so
// unlike V15 its position carries no diagnostic weight; appending keeps the
// existing rules' order untouched.
func validateToneStates(cfg DialectConfig) error {
	switch cfg.ToneStates {
	case ToneStatesCTCSS, ToneStatesCTCSSAndDCS:
		return nil
	default:
		return fmt.Errorf("cat: ToneStates is %v, which is not a domain — declare ToneStatesCTCSS or ToneStatesCTCSSAndDCS explicitly (P8 prints three states on some radios and five on others, and a state this dialect cannot express must be refused rather than encoded)", cfg.ToneStates)
	}
}
