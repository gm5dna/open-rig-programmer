// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// THE WALK, AND ITS COST, WRITTEN DOWN.
//
//	| Mode                    | What it reads                | Frames |
//	| Default (bounded)       | display groups G01-G10        |  1 000 |
//	| WithFullInventoryWalk() | all 100 groups x 100 channels | 10 000 |
//
// Both walk in ascending display order and both abort honestly on any
// error that is not "this slot is empty".
//
// THE DEFAULT BOUND OF TEN GROUPS IS A CHOICE, ARGUED: the radio's own
// front panel fills groups from the bottom, the budget is 500 channels
// against 10 000 addresses so the space is sparse by construction, and a
// user whose memories sit above group 10 has the flag. The full walk costs
// MINUTES at any plausible rate, which is why it is not the default.
//
// THERE IS NO EARLY STOP, IN EITHER MODE. An earlier draft had both modes
// stop after a run of consecutive empty slots and named no threshold;
// any threshold silently truncates a within-group gap larger than itself,
// and a truncation this table does not state is exactly the class of
// defect this driver refuses elsewhere. The frame bound already caps the
// default walk's cost, so early stopping buys nothing the bound does not
// already buy. Both modes read EVERY address in their range.
//
// THE ONE TRUNCATION THIS DESIGN HAS is the one the table states — groups
// above ten, in the default mode — and ruling T3 is what makes it safe
// rather than silent: a write that ADDS a channel at a slot the walk never
// visited is REFUSED if the pre-write read finds a record there
// (write.go), naming WithFullInventoryWalk() as the remedy. The bounded
// default therefore costs refusals, never overwritten channels.
const (
	// defaultWalkGroups is how many DISPLAY groups the bounded walk
	// covers: G01…G10.
	defaultWalkGroups = 10
	// fullWalkGroups is the whole space, and it is memGroups rather than
	// a second literal so the two cannot drift.
	fullWalkGroups = memGroups
)

// discoverInventory reads every address in the walk's range and returns
// the display slots that answered with a record, in ascending display
// order.
//
// "EMPTY" MEANS EXACTLY ONE THING IN THIS DRIVER, which is why this walks
// through readRaw rather than asking the engine itself: the FA and the
// all-0xFF record are recognised in one place (read.go), so a slot this
// walk calls empty is precisely a slot ReadChannel would return an empty
// channel for.
//
// IT ABORTS ON A REAL ERROR RATHER THAN SKIPPING IT. A wrong-length record
// mid-walk fails the open with civ's ErrRecordLength — the length
// fingerprint is continuous (spec D3.2) — and a transport failure fails it
// too. Half a walk is not a smaller inventory; it is an unknown one, and a
// session that reported one as the other would hand core/clone a
// truncated image of the radio wearing a complete image's clothes.
func (s *Session) discoverInventory(ctx context.Context, full bool) ([]string, error) {
	groups := defaultWalkGroups
	if full {
		groups = fullWalkGroups
	}
	var found []string
	for g := 1; g <= groups; g++ {
		for c := 1; c <= memPerGroup; c++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			slot := spec.SparseSlot(g, c)
			addr, _, err := slotToAddress(slot)
			if err != nil {
				return nil, fmt.Errorf("ic705: inventory: %w", err)
			}
			raw, err := s.readRaw(ctx, addr)
			if err != nil {
				return nil, fmt.Errorf("ic705: inventory: slot %s: %w", slot, err)
			}
			if raw.empty {
				continue
			}
			found = append(found, slot)
		}
	}
	return found, nil
}

// materialiseInventory performs the walk and folds its result into THIS
// SESSION's capability set: the sparse memory bank's Slots becomes the
// list of channels this radio was observed to hold.
//
// THAT IS THE WHOLE POINT OF THE TASK. spec.Bank.Sparse's contract is that
// Slots lists what a read MATERIALISED, and core/clone's ReadAll iterates
// Banks[i].Slots and nothing else — so a session that never materialised
// anything would answer a whole-radio read with the four call channels and
// ZERO memories, which is what an earlier draft of this plan would have
// shipped.
//
// It happens ONCE, in Open, into the session's own copy. The package-level
// baseline is never touched — Driver.Capabilities() keeps reporting Slots
// nil, because the static value describes the MODEL, before any radio has
// been probed.
func (s *Session) materialiseInventory(ctx context.Context, full bool) error {
	slots, err := s.discoverInventory(ctx, full)
	if err != nil {
		return err
	}

	// The session's own copy, so nothing this writes can be observed
	// through the driver's baseline or through a set already handed to a
	// caller.
	caps := cloneCapabilities(s.caps)
	for i := range caps.Banks {
		if caps.Banks[i].ID == spec.BankMemory {
			caps.Banks[i].Slots = slots
		}
	}
	s.caps = caps

	s.inventory = make(map[string]bool, len(slots))
	for _, slot := range slots {
		s.inventory[slot] = true
	}
	s.info.InventoryWalk = "bounded"
	if full {
		s.info.InventoryWalk = "full"
	}
	s.info.InventorySlots = len(slots)
	return nil
}

// inventoryKnows reports whether the open-time walk actually VISITED slot
// and found a record there.
//
// IT IS RULING T3's PRECONDITION, and the reason the materialised set is
// retained rather than merged and forgotten. A capability set cannot
// answer this question: a sparse bank's Slots lists what was found, and
// "not in the list" collapses two very different situations — a slot the
// walk visited and found empty, and a slot beyond the bounded walk's range
// that was never looked at. The write path must distinguish them, because
// only the second can hide a record the user is about to overwrite.
func (s *Session) inventoryKnows(slot string) bool { return s.inventory[slot] }
