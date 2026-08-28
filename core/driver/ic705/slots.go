// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// THE TWO SLOT NAMESPACES, and the one rule that keeps them apart.
//
//	| Bank | Display strings         | Wire                      |
//	| MEM  | G01-001  … G100-100     | group 0-99,  channel 0-99 |
//	| CALL | G101-001 … G101-004     | group 100,   channel 0-3  |
//
// ONE RULE, BOTH BANKS: wire = display − 1, for the group and for the
// channel alike. G101 lies OUTSIDE MEM's addressable space (spec.Bank's
// WithinSpace requires group ≤ Groups, and MEM declares 100), so no string
// can name two banks — which is not a nicety: codeplug.Channel carries no
// bank identifier and codeplug's bankForSlot resolves by linear scan, so a
// colliding string would resolve silently to whichever bank was listed
// first. An earlier draft of this plan gave CALL the strings G100-00n and
// collided with MEM's own G100 group for exactly that reason.
//
// The radio's OWN printed numbering — group 0100, channels 0000-0003,
// "144 C1/C2" and "430 C1/C2" (matrix §1b) — is DISPLAY COSMETICS, deferred
// per spec D4 adjudication 14 and recorded in doc.go. What this file
// defines is the canonical wire-form slot string, in spec.SparseSlot's one
// spelling and no other.
const (
	// memGroups and memPerGroup are the MEM bank's DISPLAY space: 100
	// groups of 100 channels, the manual's 0000~0099 twice over.
	memGroups   = 100
	memPerGroup = 100
	// callDisplayGroup is CALL's display group number, one past MEM's
	// last, and callWireGroup is the wire index it maps to — the manual's
	// printed group 0100.
	callDisplayGroup = 101
	callWireGroup    = 100
	// callChannels is how many call channels this radio has (matrix §1b:
	// 0000-0003).
	callChannels = 4
)

// slotToAddress maps a canonical display slot string to the wire address
// this radio's CI-V memory commands carry, and to the bank the string
// belongs to.
//
// IT RETURNS THE BANK, and must: the caller's next question is always
// "which bank's capabilities govern this write?", and a mapping that
// answered only the address would leave that question to a second,
// independent piece of string parsing that could disagree with this one.
//
// A CALL CHANNEL ABOVE FOUR IS REFUSED HERE, BEFORE ANY BUILDER. civ
// carries ONE channel range per profile (0..99 for this radio, because
// narrowing it to 0..3 would make 96 of every MEM group's channels
// unaddressable), so the outbound gate itself would admit a CALL address
// with channel 4-99 — a slot the manual does not document. O-9 was ruled
// DEFERRED on 24/08/2026: E4 does not grow a per-group cap
// mid-implementation, and this refusal, WriteChannel's bank check and
// doc.go's recording of the residual gate width carry it for this wave.
// TestCallChannelsAboveFourAreRefusedBeforeAnyBuilder sweeps the whole
// range rather than sampling it.
func slotToAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	g, c, ok := spec.ParseSparseSlot(slot)
	if !ok {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic705: slot %q is not a canonical group-addressed slot: this radio's memories are G01-001…G100-100 and its call channels G101-001…G101-004", slot)
	}
	switch {
	case g == callDisplayGroup:
		if c < 1 || c > callChannels {
			return civ.ChannelAddress{}, "", fmt.Errorf("ic705: slot %q names call channel %d, but this radio's call group holds only channels 0000-0003 (four of them, displayed G101-001…G101-004)", slot, c)
		}
		return civ.ChannelAddress{Group: callWireGroup, Channel: c - 1}, spec.BankCall, nil
	case g >= 1 && g <= memGroups:
		if c < 1 || c > memPerGroup {
			return civ.ChannelAddress{}, "", fmt.Errorf("ic705: slot %q names channel %d, but each memory group holds channels 0000-0099 (displayed 001-100)", slot, c)
		}
		return civ.ChannelAddress{Group: g - 1, Channel: c - 1}, spec.BankMemory, nil
	default:
		return civ.ChannelAddress{}, "", fmt.Errorf("ic705: slot %q names group %d: this radio has memory groups G01…G100 and the call group G101", slot, g)
	}
}

// addressToSlot maps a wire address back to its canonical display slot
// string — the exact inverse of slotToAddress over the 10 004 addresses
// that have one.
//
// AN ADDRESS OUTSIDE BOTH BANKS HAS NO DISPLAY FORM, and is refused rather
// than rendered. Inventing one (a "G101-005" for wire {100, 4}, say) would
// hand a caller a slot string no bank holds, which codeplug would then
// treat as belonging to no bank at all — a refusal several layers away from
// the address that caused it.
func addressToSlot(a civ.ChannelAddress) (string, error) {
	switch {
	case a.Group == callWireGroup:
		if a.Channel < 0 || a.Channel >= callChannels {
			return "", fmt.Errorf("ic705: wire address %v is in the call group but channel %d is outside its documented 0000-0003", a, a.Channel)
		}
		return spec.SparseSlot(callDisplayGroup, a.Channel+1), nil
	case a.Group >= 0 && a.Group < memGroups:
		if a.Channel < 0 || a.Channel >= memPerGroup {
			return "", fmt.Errorf("ic705: wire address %v names channel %d, outside every memory group's 0000-0099", a, a.Channel)
		}
		return spec.SparseSlot(a.Group+1, a.Channel+1), nil
	default:
		return "", fmt.Errorf("ic705: wire address %v names group %d, which is neither a memory group (0-99) nor the call group (100)", a, a.Group)
	}
}
