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
	case ToneModeOff, ToneModeCTCSS, ToneModeCTCSSSquelch, ToneModeCTCSSRxSquelch, ToneModeDTCS, ToneModeCross:
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
//   - CTCSSToneRange, if declared, must not accompany a non-empty
//     CTCSSTones, and must be internally coherent: positive bounds, a
//     positive step, no inversion, and a maximum a whole number of steps
//     reaches. See toneRangeProblems. A radio declaring NEITHER is not a
//     problem here — Capabilities.AdmitsTone fails closed for it, which
//     is the behaviour this project wants and the behaviour an empty
//     CTCSSTones has always had.
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
//     half WHENEVER ANY BANK REACHES THE CORRESPONDING FIELD. The
//     non-empty rule moved from the Yaesu half alone to the pair,
//     because the two vocabularies never coexist on one model; the
//     problem string for "neither" is unchanged. The bank condition is
//     E5b: a model whose bank legitimately carries no shift or duplex
//     field at all — an HF-only Icom memory bank — used to be refused by
//     a rule written when every registered radio had one. Fail-closed is
//     preserved through the FIELD's own support grades: a bank that
//     reaches the field must still name the values it can hold, so every
//     Yaesu model (all four declare FieldShift and FieldCTCSSState) is
//     judged exactly as before.
//   - DuplexOptions and ToneModes get the blank/duplicate rules every
//     vocabulary gets and must carry declared (never zero-value)
//     semantics. A semantic value expressed by MORE THAN ONE entry must
//     have EXACTLY ONE of them marked Canonical (E5) — see
//     canonicalProblems. This replaces the at-most-one rule those two
//     lists used to share with ShiftOptions: multiplicity is real, and
//     what the reverse mapping needs is a single ANSWER, not a single
//     entry.
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
	if c.Transmit != HasTransmitter && c.Transmit != ReceiveOnly {
		problems = append(problems, "Transmit must be declared as HasTransmitter or ReceiveOnly")
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
		// adjudication 7; additions D3.4 and Erratum 2): Sparse, Groups,
		// PerGroup, exactly one of Budget/BudgetUnstated, and the
		// GroupBase/ChannelBase pair are legal only together, and all zero
		// without Sparse. Every bank registered before the Icom tier
		// leaves them at their zero values and so contributes nothing here.
		problems = append(problems, b.sparseProblems()...)
		if c.Transmit == ReceiveOnly {
			// The set is DERIVED (see transmitFields, which
			// IsTransmitField answers for callers outside this
			// package), not restated: a two-item literal here would
			// have let a transmit-only Field added later go ungraded
			// on a receiver. This site ranges over the declaration
			// itself rather than asking the predicate, because it
			// needs the set in a FIXED ORDER — b.Fields is a map, and
			// walking it would make the order of these problems
			// depend on the runtime. Pinned by
			// TestTransmitFields_MatchTheDeclaredMarker.
			for _, field := range transmitFields {
				if b.Fields[field] != (FieldSupport{}) {
					problems = append(problems, fmt.Sprintf("bank %s field %s must have zero FieldSupport on a ReceiveOnly radio", b.ID, field))
				}
			}
		}

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

	problems = append(problems, c.toneRangeProblems()...)

	for i := 1; i < len(c.DTCSCodes); i++ {
		if c.DTCSCodes[i-1] >= c.DTCSCodes[i] {
			problems = append(problems, fmt.Sprintf("DTCSCodes is not strictly ascending at index %d (%d >= %d)", i, c.DTCSCodes[i-1], c.DTCSCodes[i]))
		}
	}
	for i := 1; i < len(c.AttenuatorDB); i++ {
		if c.AttenuatorDB[i-1] >= c.AttenuatorDB[i] {
			problems = append(problems, fmt.Sprintf("AttenuatorDB is not strictly ascending at index %d (%d >= %d)", i, c.AttenuatorDB[i-1], c.AttenuatorDB[i]))
		}
	}
	problems = append(problems, c.programTuningStepRangeProblems()...)

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
	if len(c.ShiftOptions) == 0 && len(c.DuplexOptions) == 0 && c.anyBankReaches(FieldShift, FieldDuplex) {
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
	if len(c.CTCSSStates) == 0 && len(c.ToneModes) == 0 && c.anyBankReaches(FieldCTCSSState, FieldToneMode) {
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
	// THE CANONICAL RULE (E5) replaces "one option per direction" here.
	// core/csvio still asks for THE option with a given Direction and
	// still needs exactly one answer — but a model may genuinely express
	// one direction with two wire codes, and refusing that shape refused
	// the radio rather than the mistake. So multiplicity is admitted and
	// the ambiguity is resolved by declaration: where a direction is
	// expressed more than once, exactly one of those options carries
	// Canonical.
	problems = append(problems, canonicalProblems("DuplexOptions", "direction", duplexCanonicalGroups(c.DuplexOptions))...)

	problems = append(problems, validateVocabEntries("ToneModes", toneModeValues(c.ToneModes))...)
	for _, m := range c.ToneModes {
		if !validToneModeSemantics(m.Semantics) {
			problems = append(problems, fmt.Sprintf("ToneModes %q has invalid Semantics %d", m.Value, m.Semantics))
		}
		if c.Transmit == ReceiveOnly && m.NeedsTxTone() {
			problems = append(problems, fmt.Sprintf("ToneModes %q has NeedsTxTone semantics on a ReceiveOnly radio", m.Value))
		}
	}
	// The canonical rule again, on the other Icom vocabulary.
	problems = append(problems, canonicalProblems("ToneModes", "semantics", toneModeCanonicalGroups(c.ToneModes))...)

	problems = append(problems, validateVocabEntries("DTCSPolarities", c.DTCSPolarities)...)
	problems = append(problems, validateVocabEntries("Filters", c.Filters)...)
	problems = append(problems, validateVocabEntries("TuningSteps", c.TuningSteps)...)
	problems = append(problems, validateVocabEntries("PreampOptions", c.PreampOptions)...)
	problems = append(problems, validateVocabEntries("AntennaOptions", c.AntennaOptions)...)

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("spec: Capabilities.Validate: %s", strings.Join(problems, "; "))
}

// programTuningStepRangeProblems checks the optional D8 programmable-step
// domain. Both bounds are aligned to zero, not merely to each other: they
// are complete step values in hertz, and the radio's resolution describes
// which absolute values it can encode.
func (c Capabilities) programTuningStepRangeProblems() []string {
	r := c.ProgramTuningStepRange
	if r == nil {
		return nil
	}
	var problems []string
	if r.ResolutionHz == 0 {
		problems = append(problems, "ProgramTuningStepRange.ResolutionHz 0 must be greater than zero")
	}
	if r.MinHz > r.MaxHz {
		problems = append(problems, fmt.Sprintf("ProgramTuningStepRange.MinHz %d is greater than MaxHz %d", r.MinHz, r.MaxHz))
	}
	if r.ResolutionHz > 0 && r.MinHz%r.ResolutionHz != 0 {
		problems = append(problems, fmt.Sprintf("ProgramTuningStepRange.MinHz %d is not aligned to ResolutionHz %d", r.MinHz, r.ResolutionHz))
	}
	if r.ResolutionHz > 0 && r.MaxHz%r.ResolutionHz != 0 {
		problems = append(problems, fmt.Sprintf("ProgramTuningStepRange.MaxHz %d is not aligned to ResolutionHz %d", r.MaxHz, r.ResolutionHz))
	}
	return problems
}

// toneRangeProblems checks the optional numeric tone domain
// (Capabilities.CTCSSToneRange). It contributes NOTHING when the field is
// nil, which is every radio registered before the Icom tier.
//
// BOTH-AT-ONCE IS THE RULE THAT MATTERS. A radio declaring a list AND a
// range has stated its tone domain twice, and the two statements can
// disagree; Capabilities.AdmitsTone would then consult one of them and
// silently pick a winner, which is exactly the "answer depends on which
// field the reader happened to look at" failure the vocabulary uniqueness
// rules elsewhere in this function exist to prevent.
//
// THE OTHER THREE ARE COHERENCE. An inverted range admits nothing while
// looking like a declaration; a non-positive bound is not a frequency; a
// non-positive step describes no spacing and would divide by zero in
// ToneRange.admits, which refuses rather than panics precisely because a
// hand-built Capabilities can reach it without passing through here.
//
// THE MAXIMUM MUST BE REACHABLE, and that one is a judgement rather than
// an arithmetic necessity. A range of 67.0 to 68.5 Hz in 1.0 Hz steps
// does not admit 68.5, so the author has written down a bound their radio
// does not have — harmless to the predicate, which simply never returns
// true for it, and a sign the transcription is wrong. It is refused so
// the mistake is found at construction rather than as a tone the UI
// offers and the radio rejects.
func (c Capabilities) toneRangeProblems() []string {
	r := c.CTCSSToneRange
	if r == nil {
		return nil
	}
	var problems []string
	if len(c.CTCSSTones) > 0 {
		problems = append(problems, fmt.Sprintf("both CTCSSTones (%d entries) and CTCSSToneRange are declared; a radio's tone domain is a list OR a range, never both", len(c.CTCSSTones)))
	}
	if r.MinDeciHz <= 0 {
		problems = append(problems, fmt.Sprintf("CTCSSToneRange.MinDeciHz %d must be greater than zero", int(r.MinDeciHz)))
	}
	if r.MaxDeciHz <= 0 {
		problems = append(problems, fmt.Sprintf("CTCSSToneRange.MaxDeciHz %d must be greater than zero", int(r.MaxDeciHz)))
	}
	if r.StepDeciHz <= 0 {
		problems = append(problems, fmt.Sprintf("CTCSSToneRange.StepDeciHz %d must be greater than zero", int(r.StepDeciHz)))
	}
	if r.MinDeciHz > r.MaxDeciHz {
		problems = append(problems, fmt.Sprintf("CTCSSToneRange.MinDeciHz %v is greater than MaxDeciHz %v", r.MinDeciHz, r.MaxDeciHz))
	}
	if r.StepDeciHz > 0 && r.MinDeciHz <= r.MaxDeciHz && (r.MaxDeciHz-r.MinDeciHz)%r.StepDeciHz != 0 {
		problems = append(problems, fmt.Sprintf("CTCSSToneRange.MaxDeciHz %v is not a whole number of StepDeciHz (%v) above MinDeciHz (%v), so the declared maximum is not itself an admissible tone", r.MaxDeciHz, r.StepDeciHz, r.MinDeciHz))
	}
	return problems
}

// anyBankReaches reports whether ANY bank in c can reach at least one of
// the given fields — that is, declares it with a support grade other than
// Unsupported in both directions.
//
// IT IS E5b's WHOLE MECHANISM. "This model has no repeater shift" and
// "this model's author forgot to declare its shift vocabulary" used to be
// the same Capabilities value, and Validate refused both to be safe. They
// are distinguishable after all, and the distinguishing fact is the one a
// driver already states: whether a bank reaches the FIELD. A bank that
// reaches FieldShift or FieldDuplex must name the values that field can
// hold; a bank that reaches neither has nothing to name, and demanding a
// vocabulary for it would force an author to invent one.
//
// FAIL-CLOSED IS PRESERVED BY THE GRADES THEMSELVES. A field left out of a
// bank's map, or present as the zero FieldSupport, answers Unreachable —
// and an Unreachable field is never read into a codeplug and never written
// to a radio (FieldSupport.CanWrite is false either way), so there is no
// path by which a missing vocabulary could be consulted.
func (c Capabilities) anyBankReaches(fields ...Field) bool {
	for _, b := range c.Banks {
		for _, f := range fields {
			if !b.Fields[f].Unreachable() {
				return true
			}
		}
	}
	return false
}

// canonicalGroup is one semantic value's entries, for the canonical rule:
// the wire codes expressing it, and how many of them are marked canonical.
type canonicalGroup struct {
	// semantic renders the shared semantic value for a diagnostic.
	semantic string
	// values are the wire codes expressing it, in declaration order.
	values []string
	// canonical counts how many of them carry Canonical.
	canonical int
}

// canonicalProblems applies E5's rule to one vocabulary's grouped entries:
// where a semantic value is expressed MORE THAN ONCE, exactly one of those
// entries must be marked Canonical.
//
// A LONE ENTRY IS EXEMPT, deliberately. There is nothing to choose between
// when a semantic has one wire code, and requiring the flag anyway would
// put a mandatory `Canonical: true` on every line of every model's table —
// ceremony that carries no information and that an author would soon apply
// without reading. The reverse mapping is written to match: it prefers a
// canonical entry and falls back to a lone one, and refuses to guess
// between two unmarked ones, which after this rule is unreachable.
//
// THE TWO FAILURES ARE NAMED SEPARATELY because they are different
// mistakes. "No canonical" is an author who did not notice the
// multiplicity; "more than one canonical" is an author who did and then
// answered the question twice.
func canonicalProblems(list, what string, groups []canonicalGroup) []string {
	var problems []string
	for _, g := range groups {
		if len(g.values) < 2 {
			continue
		}
		switch {
		case g.canonical == 0:
			problems = append(problems, fmt.Sprintf("%s entries %v share the %s %s and no canonical one is marked — the reverse mapping would otherwise depend on the order they were declared in", list, g.values, what, g.semantic))
		case g.canonical > 1:
			problems = append(problems, fmt.Sprintf("%s entries %v share the %s %s and more than one canonical one is marked — exactly one entry answers \"which wire code means this?\"", list, g.values, what, g.semantic))
		}
	}
	return problems
}

// duplexCanonicalGroups groups options by Direction, preserving both
// declaration order within a group and first-appearance order between
// groups, so a Validate message never depends on map iteration.
func duplexCanonicalGroups(options []DuplexOption) []canonicalGroup {
	index := make(map[DuplexDirection]int, len(options))
	var groups []canonicalGroup
	for _, o := range options {
		i, seen := index[o.Direction]
		if !seen {
			i = len(groups)
			index[o.Direction] = i
			groups = append(groups, canonicalGroup{semantic: fmt.Sprintf("%d", o.Direction)})
		}
		groups[i].values = append(groups[i].values, o.Value)
		if o.Canonical {
			groups[i].canonical++
		}
	}
	return groups
}

// toneModeCanonicalGroups is duplexCanonicalGroups for ToneModes.
func toneModeCanonicalGroups(modes []ToneMode) []canonicalGroup {
	index := make(map[ToneModeSemantics]int, len(modes))
	var groups []canonicalGroup
	for _, m := range modes {
		i, seen := index[m.Semantics]
		if !seen {
			i = len(groups)
			index[m.Semantics] = i
			groups = append(groups, canonicalGroup{semantic: fmt.Sprintf("%d", m.Semantics)})
		}
		groups[i].values = append(groups[i].values, m.Value)
		if m.Canonical {
			groups[i].canonical++
		}
	}
	return groups
}

// CanonicalDuplexOption returns the wire-form duplex value this radio uses
// for direction d, and true, or ("", false) when the table gives no single
// answer for it.
//
// IT SCANS THE WHOLE GROUP BEFORE ANSWERING, and that is the point rather
// than an inefficiency. Returning on the FIRST canonical entry would be
// correct for every Capabilities that passed Validate — which refuses two
// canonicals for one value — and would quietly reintroduce order
// dependence for every Capabilities that did not. That is not a
// theoretical set: core/csvio's ImportCHIRP accepts a spec.Capabilities
// and does not re-run Validate, so a hand-built or test-built table
// reaches this lookup exactly as written. "Which wire code means DOWN?"
// would once again be answered by whichever line the author happened to
// type first — the precise failure the canonical rule was introduced to
// remove.
//
// So the answer is given only when it is unambiguous:
//
//   - exactly one entry marked Canonical among those sharing d — the
//     normal multi-code case;
//   - or exactly one entry with d at all, canonical or not — a lone entry
//     needs no marking, there being nothing to choose between.
//
// Everything else — no entry, several with no canonical, several with
// more than one canonical — is not an answer this function will invent.
func (c Capabilities) CanonicalDuplexOption(d DuplexDirection) (string, bool) {
	var canonical, sole string
	var canonicals, total int
	for _, o := range c.DuplexOptions {
		if o.Direction != d {
			continue
		}
		total++
		sole = o.Value
		if o.Canonical {
			canonicals++
			canonical = o.Value
		}
	}
	switch {
	case canonicals == 1:
		return canonical, true
	case canonicals == 0 && total == 1:
		return sole, true
	default:
		return "", false
	}
}

// CanonicalToneMode is CanonicalDuplexOption for the tone-mode
// vocabulary, on identical terms and for identical reasons.
func (c Capabilities) CanonicalToneMode(s ToneModeSemantics) (string, bool) {
	var canonical, sole string
	var canonicals, total int
	for _, m := range c.ToneModes {
		if m.Semantics != s {
			continue
		}
		total++
		sole = m.Value
		if m.Canonical {
			canonicals++
			canonical = m.Value
		}
	}
	switch {
	case canonicals == 1:
		return canonical, true
	case canonicals == 0 && total == 1:
		return sole, true
	default:
		return "", false
	}
}
