// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"fmt"
	"sort"
	"strings"
)

// validSupport reports whether s is one of the five declared Support
// constants (Inert included — HW-CONFIRMED at M5b as a real state a
// field can be in; ConsentedUnverified included too, though only a write
// label may carry it — see the read-side check in Validate). A Support
// value constructed any other way (e.g. Support(99) from a hand-built or
// corrupted Capabilities) is out of range and must never reach
// FieldSupport.CanWrite's write-state comparison silently — Validate is
// what catches that before it does.
func validSupport(s Support) bool {
	switch s {
	case Unsupported, Unverified, Supported, Inert, ConsentedUnverified:
		return true
	default:
		return false
	}
}

// containsInt reports whether v appears in list.
func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// validateVocab checks that a capability vocabulary list — ShiftOptions,
// or CTCSSStates' Values — is non-empty and contains no blank or
// duplicate entries. fieldName names the field for the returned problem
// strings, e.g. "ShiftOptions".
func validateVocab(fieldName string, values []string) []string {
	var problems []string
	if len(values) == 0 {
		problems = append(problems, fmt.Sprintf("%s must not be empty", fieldName))
	}
	return append(problems, validateVocabEntries(fieldName, values)...)
}

// validateVocabEntries is validateVocab WITHOUT the non-empty rule: the
// blank and duplicate checks alone. It exists for the Icom tier's paired
// vocabularies (design D4), where an EMPTY list is a legitimate positive
// statement — "this radio expresses no such vocabulary" — as long as the
// other half of the pair is present. Validate applies the non-empty rule
// to the pair, not to each member.
func validateVocabEntries(fieldName string, values []string) []string {
	var problems []string
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if v == "" {
			problems = append(problems, fmt.Sprintf("%s must not contain a blank value", fieldName))
			continue
		}
		if seen[v] {
			problems = append(problems, fmt.Sprintf("%s contains duplicate value %q", fieldName, v))
			continue
		}
		seen[v] = true
	}
	return problems
}

// shiftOptionValues returns the Value of every entry in opts, in order —
// so validateVocab can check a ShiftOption list with the same blank and
// duplicate rules it applies to every other vocabulary, without a
// []string being built by hand at the call site.
func shiftOptionValues(opts []ShiftOption) []string {
	values := make([]string, len(opts))
	for i, o := range opts {
		values[i] = o.Value
	}
	return values
}

// validShiftDirection reports whether d is one of the three declared,
// meaningful ShiftDirection constants. ShiftUnspecified (the zero value)
// is deliberately excluded: a ShiftOption whose Direction was simply
// never set must fail here, not silently read as ShiftNone — see
// ShiftDirection's doc comment.
func validShiftDirection(d ShiftDirection) bool {
	switch d {
	case ShiftNone, ShiftUp, ShiftDown:
		return true
	default:
		return false
	}
}

// duplexOptionValues returns the Value of every entry in opts, in order,
// so validateVocabEntries can check a DuplexOption list with the same
// blank and duplicate rules every other vocabulary gets.
func duplexOptionValues(opts []DuplexOption) []string {
	values := make([]string, len(opts))
	for i, o := range opts {
		values[i] = o.Value
	}
	return values
}

// toneModeValues returns the Value of every entry in modes, in order.
func toneModeValues(modes []ToneMode) []string {
	values := make([]string, len(modes))
	for i, m := range modes {
		values[i] = m.Value
	}
	return values
}

// validDuplexDirection reports whether d is one of the three declared,
// meaningful DuplexDirection constants. DuplexUnspecified (the zero
// value) is deliberately excluded, exactly as ShiftUnspecified is.
func validDuplexDirection(d DuplexDirection) bool {
	switch d {
	case DuplexOff, DuplexUp, DuplexDown:
		return true
	default:
		return false
	}
}

// validToneModeSemantics reports whether s is one of the five declared,
// meaningful ToneModeSemantics constants. ToneModeUnspecified (the zero
// value) is deliberately excluded.
func validToneModeSemantics(s ToneModeSemantics) bool {
	switch s {
	case ToneModeOff, ToneModeCTCSS, ToneModeCTCSSSquelch, ToneModeDTCS, ToneModeCross:
		return true
	default:
		return false
	}
}

// validToneSemantics reports whether s is one of the three declared,
// meaningful ToneSemantics constants. ToneSemanticsUnspecified (the zero
// value) is deliberately excluded: a ToneState whose Semantics was simply
// never set must fail here, not silently read as ToneOff — see
// ToneSemantics' doc comment.
func validToneSemantics(s ToneSemantics) bool {
	switch s {
	case ToneOff, ToneEncode, ToneEncodeDecode:
		return true
	default:
		return false
	}
}

// Validate checks c for internal STRUCTURAL consistency — not hardware
// truth (proving a field actually works on real hardware is what M5b
// verification sessions are for), but the basic shape guarantees generic
// code (UI, validation, the clone service) is entitled to assume without
// re-checking every time it reads a Capabilities value. It never mutates
// c.
//
// Checks (every problem found is reported together in one error, not
// just the first):
//
//   - Model and CATID must both be non-empty.
//   - TagLen must be greater than zero.
//   - No two Banks may share a BankID.
//   - No slot (Bank.Slots entry) may be claimed by more than one Bank.
//   - No slot (Bank.Slots entry) may be blank: a blank slot is not a
//     real canonical wire-form identifier, and core/csvio's CHIRP
//     importer would otherwise build a Channel{Slot: ""} for it with no
//     blocking loss entry to catch the mistake.
//   - Every FieldSupport.Read and .Write across every Bank's Fields must
//     be one of the five declared Support constants (see validSupport)
//     — a value constructed any other way must never reach
//     FieldSupport.CanWrite's comparison unnoticed.
//   - No FieldSupport.Read may be ConsentedUnverified: consent is a
//     write-side state, so a read label carrying it is a construction
//     mistake, not a description of the radio.
//   - MinFreqHz must not exceed MaxFreqHz, but ONLY when both are set
//     (non-zero): either being the zero value means "no bound", not "zero
//     Hz", so it is not compared.
//   - Every entry in Bauds must be greater than zero, and DefaultBaud
//     must be greater than zero: a non-positive entry cannot be a real
//     serial baud rate, and core/transport.OpenSerial treats any
//     non-positive SerialConfig.Baud as "unset" and silently substitutes
//     its own DefaultBaud (38400) — Validate must catch a bogus baud
//     here, before that substitution can happen unnoticed.
//   - DefaultBaud must appear in Bauds.
//   - CTCSSTones, if non-empty, must be strictly ascending (matching
//     StandardCTCSSTones's own shape) — this is what lets a caller
//     safely use CTCSSTones' slice index as the CAT tone number, the way
//     StandardCTCSSTones' index doubles as one.
//   - Every entry in RequiredSlots must appear in at least one Bank's Slots
//     (subset invariant): a RequiredSlot missing from all banks would
//     silently evade completeness checking.
//   - ShiftOptions must be non-empty and contain no blank or duplicate
//     values.
//   - CTCSSStates must be non-empty and contain no blank or duplicate
//     Values.
//   - Every ShiftOptions entry's Direction must be one of the three
//     declared ShiftDirection constants (ShiftNone/ShiftUp/ShiftDown) —
//     never the zero value, ShiftUnspecified: see ShiftDirection's doc
//     comment for why the zero value must not be allowed to mean
//     anything.
//   - Every CTCSSStates entry's Semantics must be one of the three
//     declared ToneSemantics constants (ToneOff/ToneEncode/
//     ToneEncodeDecode) — never the zero value, ToneSemanticsUnspecified
//     — for the same reason.
//   - No two ShiftOptions may express the same ShiftDirection.
//   - No two CTCSSStates may express the same Semantics.
//
// The Icom tier (design D4) adds five rules, every one of which is
// VACUOUS for a radio registered before it (all five fields are then
// zero/empty), so none of them can change an existing profile's verdict:
//
//   - A Bank's sparse-space descriptor must be internally consistent:
//     Sparse/Groups/PerGroup/Budget are legal only together, and all
//     three numbers must be zero when Sparse is false (see
//     Bank.sparseProblems).
//   - The two vocabulary PAIRS — ShiftOptions/DuplexOptions and
//     CTCSSStates/ToneModes — must each have at least one non-empty
//     half. The non-empty rule moved from the Yaesu half alone to the
//     pair, because the two vocabularies never coexist on one model;
//     the problem string for "neither" is unchanged.
//   - DuplexOptions and ToneModes get the blank/duplicate rules every
//     vocabulary gets, must carry declared (never zero-value) semantics,
//     and may express each semantic at most once — for the reason
//     ShiftOptions' own uniqueness rule gives.
//   - DTCSPolarities and Filters must contain no blank or duplicate
//     value.
//   - DTCSCodes, if non-empty, must be strictly ascending.
//
// There is no separate "RequiresTone must equal Encodes||Decodes"
// invariant: that used to be checked because RequiresTone, Encodes and
// Decodes were three independent stored bool fields that could disagree.
// ToneState now stores only Semantics; RequiresTone is a method fully
// derived from it (see ToneState.RequiresTone), so there is no
// independent value left for it to disagree with.
//
// Every radio driver constructor is expected to call Validate on the
// Capabilities value it builds and fail construction if it returns a
// non-nil error. A driver registry will enforce this centrally in a
// later task; nothing in this package can enforce it as a compile-time
// guarantee today, so calling this remains each constructor's own
// responsibility until then. Validate is otherwise side-effect-free and
// safe to call as many times as a caller likes, on any Capabilities
// value including a partially-built or hand-crafted test fixture.
func (c Capabilities) Validate() error {
	var problems []string

	if c.Model == "" {
		problems = append(problems, "Model must not be empty")
	}
	if c.CATID == "" {
		problems = append(problems, "CATID must not be empty")
	}
	// A TagLen of zero (or less) is not "no tag support" — core/csvio's
	// CHIRP import truncates every imported name to b[:caps.TagLen], so a
	// zero TagLen silently discards every channel name to "" and reports
	// it as an approximated, non-blocking loss rather than refusing. This
	// project's standing posture is refuse, never corrupt: a driver that
	// forgets to set TagLen must fail construction here, not reach a
	// radio having erased every tag.
	if c.TagLen <= 0 {
		problems = append(problems, fmt.Sprintf("TagLen %d must be greater than zero", c.TagLen))
	}

	seenBank := make(map[BankID]bool, len(c.Banks))
	seenSlot := make(map[string]BankID, len(c.Banks))
	for _, b := range c.Banks {
		if seenBank[b.ID] {
			problems = append(problems, fmt.Sprintf("duplicate BankID %q", b.ID))
		}
		seenBank[b.ID] = true

		// The sparse-space descriptor's own consistency (design D4,
		// adjudication 7): Sparse/Groups/PerGroup/Budget are legal only
		// together, and all zero without Sparse. Every bank registered
		// before the Icom tier leaves all four at their zero values and
		// so contributes nothing here.
		problems = append(problems, b.sparseProblems()...)

		for _, slot := range b.Slots {
			if slot == "" {
				problems = append(problems, fmt.Sprintf("bank %s has a blank slot", b.ID))
				continue
			}
			if owner, ok := seenSlot[slot]; ok {
				problems = append(problems, fmt.Sprintf("slot %q is claimed by both bank %s and bank %s", slot, owner, b.ID))
				continue
			}
			seenSlot[slot] = b.ID
		}

		// Iterate Fields in a deterministic (sorted) order: map
		// iteration order is randomised by Go, and this function's error
		// message should not be.
		fieldNames := make([]string, 0, len(b.Fields))
		for f := range b.Fields {
			fieldNames = append(fieldNames, string(f))
		}
		sort.Strings(fieldNames)
		for _, fn := range fieldNames {
			f := Field(fn)
			fs := b.Fields[f]
			if !validSupport(fs.Read) {
				problems = append(problems, fmt.Sprintf("bank %s field %s: Read support %d is out of range", b.ID, f, fs.Read))
			}
			// ConsentedUnverified is declared, so validSupport accepts it —
			// but only as a WRITE label. Reads already flow and need no
			// consent, so a read label carrying it is a construction
			// mistake (a transform applied to the wrong half of the pair),
			// and this is where it is caught.
			if fs.Read == ConsentedUnverified {
				problems = append(problems, fmt.Sprintf("bank %s field %s: Read support must never be ConsentedUnverified — consent is a write-side state; reads already flow and need no consent", b.ID, f))
			}
			if !validSupport(fs.Write) {
				problems = append(problems, fmt.Sprintf("bank %s field %s: Write support %d is out of range", b.ID, f, fs.Write))
			}
		}
	}

	if c.MinFreqHz != 0 && c.MaxFreqHz != 0 && c.MinFreqHz > c.MaxFreqHz {
		problems = append(problems, fmt.Sprintf("MinFreqHz %d is greater than MaxFreqHz %d", c.MinFreqHz, c.MaxFreqHz))
	}

	// A non-positive Bauds entry or DefaultBaud cannot be a real serial
	// baud rate: core/transport.OpenSerial's resolveConfig treats any
	// SerialConfig.Baud <= 0 as "unset" and silently substitutes its own
	// DefaultBaud (38400), so a Capabilities that let one through here
	// would have its bogus value replaced by a guess deep in the
	// transport layer, never refused.
	for _, baud := range c.Bauds {
		if baud <= 0 {
			problems = append(problems, fmt.Sprintf("Bauds contains non-positive entry %d", baud))
		}
	}
	if c.DefaultBaud <= 0 {
		problems = append(problems, fmt.Sprintf("DefaultBaud %d must be greater than zero", c.DefaultBaud))
	}

	if !containsInt(c.Bauds, c.DefaultBaud) {
		problems = append(problems, fmt.Sprintf("DefaultBaud %d is not present in Bauds %v", c.DefaultBaud, c.Bauds))
	}

	for i := 1; i < len(c.CTCSSTones); i++ {
		if c.CTCSSTones[i-1] >= c.CTCSSTones[i] {
			problems = append(problems, fmt.Sprintf("CTCSSTones is not strictly ascending at index %d (%v >= %v)", i, c.CTCSSTones[i-1], c.CTCSSTones[i]))
		}
	}

	for i := 1; i < len(c.DTCSCodes); i++ {
		if c.DTCSCodes[i-1] >= c.DTCSCodes[i] {
			problems = append(problems, fmt.Sprintf("DTCSCodes is not strictly ascending at index %d (%d >= %d)", i, c.DTCSCodes[i-1], c.DTCSCodes[i]))
		}
	}

	for _, requiredSlot := range c.RequiredSlots {
		found := false
		for _, bank := range c.Banks {
			for _, slot := range bank.Slots {
				if slot == requiredSlot {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("RequiredSlot %q not found in any bank's Slots", requiredSlot))
		}
	}

	// The repeater-shift PAIR (design D4): a radio expresses the Yaesu
	// ShiftOptions vocabulary or the Icom DuplexOptions one, never both
	// and never neither. The non-empty rule therefore applies to the
	// pair, while the blank/duplicate rules apply to each list on its
	// own. For every radio registered before the Icom tier
	// DuplexOptions is empty, so this reduces to exactly the
	// unconditional "ShiftOptions must not be empty" it replaces —
	// same problem string, same position in the list, since an empty
	// list contributes no blank/duplicate problems to reorder.
	if len(c.ShiftOptions) == 0 && len(c.DuplexOptions) == 0 {
		problems = append(problems, "ShiftOptions must not be empty")
	}
	problems = append(problems, validateVocabEntries("ShiftOptions", shiftOptionValues(c.ShiftOptions))...)

	// Every ShiftOptions entry's Direction must be a declared, meaningful
	// ShiftDirection — never ShiftUnspecified, its zero value: an option
	// whose Direction was simply omitted must be refused here, not
	// silently read as ShiftNone (see ShiftDirection's doc comment).
	for _, o := range c.ShiftOptions {
		if !validShiftDirection(o.Direction) {
			problems = append(problems, fmt.Sprintf("ShiftOptions %q has invalid Direction %d", o.Value, o.Direction))
		}
	}

	// Each ShiftDirection must be expressed by AT MOST ONE option:
	// core/csvio maps a foreign dialect's "+"/"-" by asking for the option
	// with a given Direction, and that question must have exactly one
	// answer. Two options sharing a direction would make the answer
	// depend on slice order.
	seenDirection := make(map[ShiftDirection]string, len(c.ShiftOptions))
	for _, o := range c.ShiftOptions {
		if prev, dup := seenDirection[o.Direction]; dup {
			problems = append(problems, fmt.Sprintf("ShiftOptions %q and %q express the same direction", prev, o.Value))
			continue
		}
		seenDirection[o.Direction] = o.Value
	}

	// The tone PAIR, by the same rule and for the same reason as the
	// shift pair above: CTCSSStates (Yaesu) or ToneModes (Icom).
	ctcssValues := make([]string, len(c.CTCSSStates))
	for i, ts := range c.CTCSSStates {
		ctcssValues[i] = ts.Value
	}
	if len(c.CTCSSStates) == 0 && len(c.ToneModes) == 0 {
		problems = append(problems, "CTCSSStates must not be empty")
	}
	problems = append(problems, validateVocabEntries("CTCSSStates", ctcssValues)...)

	// Every CTCSSStates entry's Semantics must be a declared, meaningful
	// ToneSemantics — never ToneSemanticsUnspecified, its zero value: a
	// state whose Semantics was simply omitted must be refused here, not
	// silently read as ToneOff (see ToneSemantics' doc comment).
	for _, ts := range c.CTCSSStates {
		if !validToneSemantics(ts.Semantics) {
			problems = append(problems, fmt.Sprintf("CTCSSStates %q has invalid Semantics %d", ts.Value, ts.Semantics))
		}
	}

	// For the same reason ShiftOptions' directions must be unique, each
	// Semantics value must name at most one state.
	seenSemantics := make(map[ToneSemantics]string, len(c.CTCSSStates))
	for _, ts := range c.CTCSSStates {
		if prev, dup := seenSemantics[ts.Semantics]; dup {
			problems = append(problems, fmt.Sprintf("CTCSSStates %q and %q express the same semantics", prev, ts.Value))
			continue
		}
		seenSemantics[ts.Semantics] = ts.Value
	}

	// The Icom-tier vocabularies (design D4). Each is checked only for
	// INTERNAL consistency when supplied — blank/duplicate values,
	// declared semantics, one option per semantic — and contributes
	// nothing at all when empty, which is every radio registered before
	// that tier.
	problems = append(problems, validateVocabEntries("DuplexOptions", duplexOptionValues(c.DuplexOptions))...)
	for _, o := range c.DuplexOptions {
		if !validDuplexDirection(o.Direction) {
			problems = append(problems, fmt.Sprintf("DuplexOptions %q has invalid Direction %d", o.Value, o.Direction))
		}
	}
	// One option per direction, for the reason ShiftOptions' own
	// uniqueness rule gives: core/csvio maps CHIRP's "+"/"-" by asking
	// for the option with a given Direction, and that question must have
	// exactly one answer.
	seenDuplex := make(map[DuplexDirection]string, len(c.DuplexOptions))
	for _, o := range c.DuplexOptions {
		if prev, dup := seenDuplex[o.Direction]; dup {
			problems = append(problems, fmt.Sprintf("DuplexOptions %q and %q express the same direction", prev, o.Value))
			continue
		}
		seenDuplex[o.Direction] = o.Value
	}

	problems = append(problems, validateVocabEntries("ToneModes", toneModeValues(c.ToneModes))...)
	for _, m := range c.ToneModes {
		if !validToneModeSemantics(m.Semantics) {
			problems = append(problems, fmt.Sprintf("ToneModes %q has invalid Semantics %d", m.Value, m.Semantics))
		}
	}
	seenToneMode := make(map[ToneModeSemantics]string, len(c.ToneModes))
	for _, m := range c.ToneModes {
		if prev, dup := seenToneMode[m.Semantics]; dup {
			problems = append(problems, fmt.Sprintf("ToneModes %q and %q express the same semantics", prev, m.Value))
			continue
		}
		seenToneMode[m.Semantics] = m.Value
	}

	problems = append(problems, validateVocabEntries("DTCSPolarities", c.DTCSPolarities)...)
	problems = append(problems, validateVocabEntries("Filters", c.Filters)...)

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("spec: Capabilities.Validate: %s", strings.Join(problems, "; "))
}
