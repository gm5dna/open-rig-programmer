// SPDX-License-Identifier: GPL-3.0-or-later

package spec

// BankID identifies a family of memory slots that share the same shape and
// rules (e.g. all ordinary memory channels, or all scan-limit pairs).
type BankID string

// The four bank families this project currently models.
const (
	// BankMemory is the ordinary memory-channel bank ("MEM").
	BankMemory BankID = "MEM"
	// BankPMS is the programmable memory scan (scan limit pair) bank
	// ("PMS").
	BankPMS BankID = "PMS"
	// Bank60m is the 60 m band channel bank ("60M").
	Bank60m BankID = "60M"
	// BankEMG is the emergency/quick-recall memory bank ("EMG").
	BankEMG BankID = "EMG"
)

// Bank describes one family of memory slots: what they are called for
// display, which wire-form slot identifiers exist, whether slots in this
// bank must always stay populated, and which Field values are supported
// (and to what degree) for slots in this bank.
//
// Bank deliberately holds slot identifiers as plain strings, not
// core/cat.Slot: core/spec imports nothing project-internal, so that the
// UI and validation layers can depend on it without pulling in a
// particular radio's wire protocol.
type Bank struct {
	// ID identifies which bank family this is.
	ID BankID
	// Label is the human-readable display name, e.g. "Memories", "Scan
	// limits (PMS)".
	Label string
	// Slots lists the canonical wire-form slot identifiers in this bank,
	// e.g. "001".."099". These are plain strings, not core/cat.Slot.
	Slots []string
	// NoBlank is true when slots in this bank must stay populated and
	// cannot be erased/left blank — e.g. PMS pairs and M-01.
	NoBlank bool
	// Fields maps each Field this bank supports to its FieldSupport. A
	// Field absent from this map is implicitly Unsupported for both read
	// and write; see Capabilities.FieldSupport for the zero-value lookup
	// helper.
	Fields map[Field]FieldSupport
}
