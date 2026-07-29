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

// maxEXComponent is the largest value an EXAddress component may hold.
//
// EXAddress.Wire renders each component with %02d, which is a MINIMUM
// width, not an exact one: a component of 100 renders three digits and
// produces a seven-byte address in a field the grammar fixes at six. The
// resulting frame is rejected by the dialect's own gate and can never be
// reconstructed by its own parser, because ParseEXAddress consumes exactly
// six digits. uint8 alone therefore does not constrain this enough.
const maxEXComponent = 99

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
		validateCATID,        // V1
		validateModeNames,    // V2
		validatePMSPairs,     // V3
		validateSpecialWires, // V4
		validateMemoryRange,  // V5
		validateSixtyRange,   // V6
		validateShadowing,    // V7
		validateEXItems,      // V8
		validateMTPolicy,     // V9
		validateClarifier,    // V10
		validateMWWriteKind,  // V11
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

// validatePMSPairs is V3.
func validatePMSPairs(cfg DialectConfig) error {
	if n := cfg.Slots.PMSPairs; n < 0 || n > 9 {
		return fmt.Errorf("cat: Slots.PMSPairs is %d, want 0..9 — the wire form's pair number is a single ASCII digit, so a larger value builds forms this dialect's own ParseSlot rejects", n)
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

// validateSixtyRange is V6: the same range rule, plus non-overlap.
//
// classifySlot tests the memory range BEFORE the 60m range, so an overlap
// is not ambiguous at runtime — memory simply wins, and every colliding
// slot is silently misclassified as an ordinary channel.
func validateSixtyRange(cfg DialectConfig) error {
	if err := validateSlotRange("Slots.Sixty", cfg.Slots.SixtyLo, cfg.Slots.SixtyHi); err != nil {
		return err
	}
	s := cfg.Slots
	if s.MemoryHi > 0 && s.SixtyHi > 0 && s.MemoryLo <= s.SixtyHi && s.SixtyLo <= s.MemoryHi {
		return fmt.Errorf("cat: memory range %d..%d overlaps 60m range %d..%d — classifySlot checks memory first, so every slot in the overlap would be classified as an ordinary channel", s.MemoryLo, s.MemoryHi, s.SixtyLo, s.SixtyHi)
	}
	return nil
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
		if pmsWireInRange(w.value, s.PMSPairs) {
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

// pmsWireInRange reports whether wire is a PMS form a dialect with pairs
// pairs would build and classify, i.e. "P<1..pairs><L|U>".
func pmsWireInRange(wire string, pairs int) bool {
	if pairs <= 0 || len(wire) != 3 {
		return false
	}
	return wire[0] == 'P' &&
		wire[1] >= '1' && wire[1] <= byte('0'+pairs) &&
		(wire[2] == 'L' || wire[2] == 'U')
}

// validateEXItems is V8.
func validateEXItems(cfg DialectConfig) error {
	seen := make(map[EXAddress]int, len(cfg.EXItems))
	for i, it := range cfg.EXItems {
		for _, c := range []struct {
			name string
			v    uint8
		}{{"P1", it.Addr.P1}, {"P2", it.Addr.P2}, {"P3", it.Addr.P3}} {
			if int(c.v) > maxEXComponent {
				return fmt.Errorf("cat: EXItems[%d].Addr.%s is %d, want <= %d — EXAddress.Wire renders %%02d, a MINIMUM width, so a larger component produces a 7-byte address in a 6-byte field", i, c.name, c.v, maxEXComponent)
			}
		}
		if prev, dup := seen[it.Addr]; dup {
			return fmt.Errorf("cat: EXItems[%d] repeats address %v, already at index %d", i, it.Addr, prev)
		}
		seen[it.Addr] = i
		if it.Digits < 1 {
			return fmt.Errorf("cat: EXItems[%d] (%v) has Digits %d, want >= 1", i, it.Addr, it.Digits)
		}
		if it.Digits > maxEXDigits {
			return fmt.Errorf("cat: EXItems[%d] (%v) has Digits %d, want <= %d — a wider P4 describes an answer frame longer than DefaultMaxFrame (%d), which this dialect's own transport could never assemble", i, it.Addr, it.Digits, maxEXDigits, DefaultMaxFrame)
		}
	}
	return nil
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
