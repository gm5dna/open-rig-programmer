// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"fmt"
	"strconv"
	"strings"
)

// BankID identifies a family of memory slots that share the same shape and
// rules (e.g. all ordinary memory channels, or all scan-limit pairs).
type BankID string

// The bank families this project currently models.
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
	// BankCall is the call-channel bank ("CALL") the Icom tier adds
	// (design D4). It is its own family rather than a corner of MEM
	// because a call channel is separately addressed on the wire.
	BankCall BankID = "CALL"
	// BankScan is the scan-edge bank ("SCAN") the Icom tier adds (design
	// D4). It is deliberately NOT mapped onto BankPMS: PMS carries the
	// Yaesu pair invariants (a lower/upper pair per index, NoBlank), and
	// a scan edge on another family is not obliged to honour them.
	BankScan BankID = "SCAN"
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

	// Sparse is true when Slots lists only the slots a read actually
	// MATERIALISED (the occupied ones, plus any the user has since
	// added) out of a much larger addressable space — the Icom tier's
	// group-addressed banks (design D4, adjudication 7). A dense bank
	// (every Yaesu bank registered before that tier) leaves this false,
	// and Slots is then the complete, fixed inventory it has always
	// been.
	//
	// The three fields below describe that addressable space, and they
	// exist so "is this slot within the space?" is decidable from the
	// Bank ALONE — no model table, no driver call — which is what lets
	// codeplug.Diff admit an add at a slot no read has ever returned.
	// They are legal ONLY together with Sparse, and must all be zero
	// when Sparse is false; Capabilities.Validate enforces both halves.
	Sparse bool
	// Groups is how many addressable groups the sparse space has, e.g.
	// 100. GroupBase says whether numbering starts at 0 or 1.
	Groups int
	// GroupBase is the radio's first group number, 0 or 1.
	GroupBase int
	// PerGroup is how many addressable channels each group holds, e.g.
	// 100. ChannelBase says whether numbering starts at 0 or 1.
	PerGroup int
	// ChannelBase is the radio's first channel number, 0 or 1.
	ChannelBase int
	// Budget is the maximum number of POPULATED slots the radio will
	// hold across this sparse bank at once, e.g. 500 — far fewer than
	// Groups*PerGroup. It is enforced at codeplug.Diff time and never
	// sent: an over-budget candidate is refused before anything reaches
	// a radio, because what an over-budget radio actually does is
	// undocumented.
	Budget int
	// BudgetUnstated records that the address space is known but the radio's
	// populated-channel capacity is not documented. It suppresses only the
	// occupancy refusal; WithinSpace remains authoritative.
	BudgetUnstated bool
}

// SparseSlot renders the canonical wire-form slot identifier for group
// and channel in a sparse bank's addressable space: "G05-012" for group
// 5, channel 12. Zero is valid for a zero-based bank. The group is zero-padded to two
// digits and the channel to three, which is exactly wide enough for the
// 100x100 spaces this tier registers and simply grows for a wider one
// (group 100 renders "G100").
//
// This form is the ONE place the group-addressed slot string is built.
// codeplug.DisplaySlot's identity fallback already passes it through
// unchanged, so no per-model display table is needed for it (design D4,
// adjudication 14).
func SparseSlot(group, channel int) string {
	return fmt.Sprintf("G%02d-%03d", group, channel)
}

// ParseSparseSlot decodes a canonical group-addressed slot string built
// by SparseSlot, returning the radio's group and channel numbers and true. It is
// STRICT: a string is accepted only when re-rendering the decoded pair
// through SparseSlot reproduces it byte for byte, so an alternative
// spelling of the same address ("G5-12", "G005-0012") is refused rather
// than silently accepted as a second name for one slot.
func ParseSparseSlot(slot string) (group, channel int, ok bool) {
	if len(slot) < 2 || slot[0] != 'G' {
		return 0, 0, false
	}
	gs, cs, found := strings.Cut(slot[1:], "-")
	if !found || gs == "" || cs == "" {
		return 0, 0, false
	}
	g, err := strconv.Atoi(gs)
	if err != nil {
		return 0, 0, false
	}
	c, err := strconv.Atoi(cs)
	if err != nil {
		return 0, 0, false
	}
	if g < 0 || c < 0 {
		return 0, 0, false
	}
	if SparseSlot(g, c) != slot {
		return 0, 0, false
	}
	return g, c, true
}

// WithinSpace reports whether slot is an address this bank can hold.
//
// For a dense bank that is exactly membership of Slots — the only
// question there has ever been. For a Sparse bank it is membership of
// Slots OR a well-formed group address (see ParseSparseSlot) inside the
// two base-aware declared ranges: a sparse bank's Slots lists what a read
// materialised, and an address outside that list is a perfectly legal
// place for the user to ADD a channel.
func (b Bank) WithinSpace(slot string) bool {
	for _, s := range b.Slots {
		if s == slot {
			return true
		}
	}
	if !b.Sparse {
		return false
	}
	g, c, ok := ParseSparseSlot(slot)
	if !ok {
		return false
	}
	return g >= b.GroupBase && g < b.GroupBase+b.Groups &&
		c >= b.ChannelBase && c < b.ChannelBase+b.PerGroup
}

// sparseProblems returns every way this Bank's sparse-space description
// is internally inconsistent, phrased for Capabilities.Validate's
// problem list. The rule is symmetric on purpose: the three descriptor
// fields are legal only TOGETHER WITH Sparse, and must all be zero
// without it — a Groups value on a dense bank is a mistake that would
// otherwise sit there meaning nothing, and a Sparse bank missing one of
// them cannot answer WithinSpace.
func (b Bank) sparseProblems() []string {
	var problems []string
	if b.Sparse {
		if b.Groups <= 0 {
			problems = append(problems, fmt.Sprintf("bank %s: Sparse bank must have Groups greater than zero, got %d", b.ID, b.Groups))
		}
		if b.PerGroup <= 0 {
			problems = append(problems, fmt.Sprintf("bank %s: Sparse bank must have PerGroup greater than zero, got %d", b.ID, b.PerGroup))
		}
		if b.GroupBase != 0 && b.GroupBase != 1 {
			problems = append(problems, fmt.Sprintf("bank %s: GroupBase %d must be 0 or 1", b.ID, b.GroupBase))
		}
		if b.ChannelBase != 0 && b.ChannelBase != 1 {
			problems = append(problems, fmt.Sprintf("bank %s: ChannelBase %d must be 0 or 1", b.ID, b.ChannelBase))
		}
		if b.Budget < 0 {
			problems = append(problems, fmt.Sprintf("bank %s: Budget must not be negative, got %d", b.ID, b.Budget))
		} else if b.Budget > 0 == b.BudgetUnstated {
			if b.Budget == 0 {
				// Keep the established phrase so callers and the landed pin see
				// the same failure while naming the newly valid alternative.
				problems = append(problems, fmt.Sprintf("bank %s: Sparse bank must have Budget greater than zero or set BudgetUnstated (exactly one), got %d/false", b.ID, b.Budget))
			} else {
				problems = append(problems, fmt.Sprintf("bank %s: Sparse bank must declare exactly one of Budget greater than zero or BudgetUnstated, got %d/true", b.ID, b.Budget))
			}
		}
		return problems
	}
	if b.Groups != 0 {
		problems = append(problems, fmt.Sprintf("bank %s: Groups %d is set on a bank that is not Sparse", b.ID, b.Groups))
	}
	if b.PerGroup != 0 {
		problems = append(problems, fmt.Sprintf("bank %s: PerGroup %d is set on a bank that is not Sparse", b.ID, b.PerGroup))
	}
	if b.GroupBase != 0 {
		problems = append(problems, fmt.Sprintf("bank %s: GroupBase %d is set on a bank that is not Sparse", b.ID, b.GroupBase))
	}
	if b.ChannelBase != 0 {
		problems = append(problems, fmt.Sprintf("bank %s: ChannelBase %d is set on a bank that is not Sparse", b.ID, b.ChannelBase))
	}
	if b.Budget != 0 {
		problems = append(problems, fmt.Sprintf("bank %s: Budget %d is set on a bank that is not Sparse", b.ID, b.Budget))
	}
	if b.BudgetUnstated {
		problems = append(problems, fmt.Sprintf("bank %s: BudgetUnstated is set on a bank that is not Sparse", b.ID))
	}
	return problems
}
