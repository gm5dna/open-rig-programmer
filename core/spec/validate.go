// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"fmt"
	"sort"
	"strings"
)

// validSupport reports whether s is one of the four declared Support
// constants (Inert included — HW-CONFIRMED at M5b as a real state a
// field can be in; see its doc comment). A Support value constructed any
// other way (e.g. Support(99) from a hand-built or corrupted
// Capabilities) is out of range and must never reach
// FieldSupport.CanWrite's == Supported comparison silently — Validate is
// what catches that before it does.
func validSupport(s Support) bool {
	switch s {
	case Unsupported, Unverified, Supported, Inert:
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
//   - No two Banks may share a BankID.
//   - No slot (Bank.Slots entry) may be claimed by more than one Bank.
//   - Every FieldSupport.Read and .Write across every Bank's Fields must
//     be one of the four declared Support constants (see validSupport)
//     — a value constructed any other way must never reach
//     FieldSupport.CanWrite's comparison unnoticed.
//   - MinFreqHz must not exceed MaxFreqHz, but ONLY when both are set
//     (non-zero): either being the zero value means "no bound", not "zero
//     Hz", so it is not compared.
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
//     Values (its RequiresTone is a plain bool, so it cannot itself be
//     invalid).
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

	seenBank := make(map[BankID]bool, len(c.Banks))
	seenSlot := make(map[string]BankID, len(c.Banks))
	for _, b := range c.Banks {
		if seenBank[b.ID] {
			problems = append(problems, fmt.Sprintf("duplicate BankID %q", b.ID))
		}
		seenBank[b.ID] = true

		for _, slot := range b.Slots {
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
			if !validSupport(fs.Write) {
				problems = append(problems, fmt.Sprintf("bank %s field %s: Write support %d is out of range", b.ID, f, fs.Write))
			}
		}
	}

	if c.MinFreqHz != 0 && c.MaxFreqHz != 0 && c.MinFreqHz > c.MaxFreqHz {
		problems = append(problems, fmt.Sprintf("MinFreqHz %d is greater than MaxFreqHz %d", c.MinFreqHz, c.MaxFreqHz))
	}

	if !containsInt(c.Bauds, c.DefaultBaud) {
		problems = append(problems, fmt.Sprintf("DefaultBaud %d is not present in Bauds %v", c.DefaultBaud, c.Bauds))
	}

	for i := 1; i < len(c.CTCSSTones); i++ {
		if c.CTCSSTones[i-1] >= c.CTCSSTones[i] {
			problems = append(problems, fmt.Sprintf("CTCSSTones is not strictly ascending at index %d (%v >= %v)", i, c.CTCSSTones[i-1], c.CTCSSTones[i]))
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

	problems = append(problems, validateVocab("ShiftOptions", c.ShiftOptions)...)

	ctcssValues := make([]string, len(c.CTCSSStates))
	for i, ts := range c.CTCSSStates {
		ctcssValues[i] = ts.Value
	}
	problems = append(problems, validateVocab("CTCSSStates", ctcssValues)...)

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("spec: Capabilities.Validate: %s", strings.Join(problems, "; "))
}
