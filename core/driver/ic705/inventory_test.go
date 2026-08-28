// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// radioHoldingSlots scripts a radio holding the golden record at each of
// the given display slots and nothing anywhere else.
func radioHoldingSlots(t *testing.T, slots ...string) *scriptedRadio {
	t.Helper()
	records := map[civ.ChannelAddress][]byte{}
	rec := goldenRecord(t)
	for _, slot := range slots {
		records[addrOf(t, slot)] = rec
	}
	return newScriptedRadio(t, radioImage{records: records})
}

// memSlots returns the session's materialised memory-bank slot list.
func memSlots(t *testing.T, sess *Session) []string {
	t.Helper()
	b, ok := sess.Capabilities().Bank(spec.BankMemory)
	if !ok {
		t.Fatal("the session has no MEM bank")
	}
	return b.Slots
}

func TestDiscoveryMaterialisesOccupiedSlotsIntoSessionCapabilities(t *testing.T) {
	// The fixture the plan names: two memories in group 1, forty-eight
	// empty slots between them, and one in group 7. Both modes read EVERY
	// address in their range, so the gap is simply walked.
	want := []string{"G01-001", "G01-050", "G07-013"}
	r := radioHoldingSlots(t, want...)
	sess := openSession(t, r)

	got := memSlots(t, sess)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the session's MEM bank lists %v, want exactly %v in ascending display order", got, want)
	}
	if n := sess.SessionInfo().InventorySlots; n != len(want) {
		t.Errorf("SessionInfo().InventorySlots = %d, want %d", n, len(want))
	}
	if mode := sess.SessionInfo().InventoryWalk; mode != "bounded" {
		t.Errorf("SessionInfo().InventoryWalk = %q, want \"bounded\"", mode)
	}

	// THE STATIC BASELINE IS NEVER MUTATED. It describes the MODEL, before
	// any radio has been probed, and internal/wiring publishes exactly it.
	if slots := New(RealHardware).Capabilities().Banks[0].Slots; len(slots) != 0 {
		t.Errorf("the driver's static Capabilities() now lists %v — a session's discovery must never reach the package's own baseline", slots)
	}
}

func TestBoundedWalkStopsAtTenGroups(t *testing.T) {
	// A record above the bounded walk's range is NOT discovered by
	// default, and IS discovered with the flag. The cost of the flag is
	// real (ten thousand exchanges against one thousand) and it is stated
	// in inventory.go's table.
	r := radioHoldingSlots(t, "G01-001", "G11-001")

	bounded := openSession(t, r)
	if got := memSlots(t, bounded); strings.Join(got, ",") != "G01-001" {
		t.Errorf("the bounded walk materialised %v, want only G01-001 — G11 is outside the first ten display groups", got)
	}
	boundedReads := r.Reads()
	if boundedReads > probeSlots+defaultWalkGroups*memPerGroup {
		t.Errorf("the bounded open read %d memories, want at most %d (the probe plus ten groups)", boundedReads, probeSlots+defaultWalkGroups*memPerGroup)
	}

	// A second radio for the full walk, so the frame counts do not mix.
	r2 := radioHoldingSlots(t, "G01-001", "G11-001")
	full := openSession(t, r2, WithFullInventoryWalk())
	if got := memSlots(t, full); strings.Join(got, ",") != "G01-001,G11-001" {
		t.Errorf("the full walk materialised %v, want both records", got)
	}
	if mode := full.SessionInfo().InventoryWalk; mode != "full" {
		t.Errorf("SessionInfo().InventoryWalk = %q, want \"full\"", mode)
	}
	if r2.Reads() <= boundedReads {
		t.Errorf("the full walk read %d memories and the bounded one %d — the full walk must read strictly more", r2.Reads(), boundedReads)
	}
}

func TestInventoryKnowsWhatItMaterialised(t *testing.T) {
	// Ruling T3's precondition. "Not in the inventory" collapses two very
	// different situations — a slot the walk VISITED and found empty, and
	// a slot beyond the walk's range that was never looked at — and the
	// write path must distinguish them, because only the second can hide a
	// record a user is about to overwrite.
	r := radioHoldingSlots(t, "G01-001", "G11-001")
	sess := openSession(t, r)
	if !sess.inventoryKnows("G01-001") {
		t.Error("inventoryKnows(\"G01-001\") is false for a slot the walk found occupied")
	}
	if sess.inventoryKnows("G01-002") {
		t.Error("inventoryKnows(\"G01-002\") is true for a slot the walk found EMPTY")
	}
	if sess.inventoryKnows("G11-001") {
		t.Error("inventoryKnows(\"G11-001\") is true for a slot the bounded walk never visited — this is exactly the case the occupied-surprise refusal exists for")
	}
}

func TestDiscoveryAbortsOnARealError(t *testing.T) {
	// A wrong-length record mid-walk fails the OPEN with ErrRecordLength.
	// It is not silently skipped as "empty": the fingerprint is continuous
	// (spec D3.2), and half a walk is not a smaller inventory, it is an
	// unknown one.
	r := radioHoldingSlots(t, "G01-001")
	r.SetRecord(addrOf(t, "G02-005"), make([]byte, 39))
	sess, err := openSessionErr(t, r)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open succeeded although a memory mid-walk answered with a 39-byte record")
	}
	if !errors.Is(err, civ.ErrRecordLength) {
		t.Errorf("Open failed with %v, want ErrRecordLength", err)
	}
}

func TestDiscoveryAbortsOnATransportFailure(t *testing.T) {
	// The same rule for the other class of failure: a radio that stops
	// answering mid-walk fails the open rather than reporting the slots it
	// managed to reach as a complete inventory.
	// The radio answers the sixteen probe reads and twenty more, then
	// falls silent: the walk meets a timeout rather than an empty slot.
	r := newScriptedRadio(t, radioImage{
		records:          map[civ.ChannelAddress][]byte{addrOf(t, "G01-001"): goldenRecord(t)},
		silentAfterReads: probeSlots + 20,
	})
	sess, err := openSessionErr(t, r)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open succeeded although the radio went silent mid-walk")
	}
	if !strings.Contains(err.Error(), "inventory") {
		t.Errorf("Open failed with %v, which does not say the inventory walk was what failed", err)
	}
}

func TestCloneReadAllReturnsTheMemories(t *testing.T) {
	// THE POINT OF THE TASK. core/clone's ReadAll iterates
	// Capabilities().Banks[i].Slots and nothing else, and a sparse bank's
	// Slots is what a read MATERIALISED — so without the walk this returns
	// the four call channels and ZERO memories.
	r := radioHoldingSlots(t, "G01-001", "G01-050", "G07-013",
		"G101-001", "G101-002", "G101-003", "G101-004")
	sess := openSession(t, r)

	svc := clone.NewService(sess, clone.SnapshotStore{Dir: t.TempDir()})
	cp, err := svc.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("clone.ReadAll: %v", err)
	}
	if len(cp.Channels) != 7 {
		var slots []string
		for _, ch := range cp.Channels {
			slots = append(slots, ch.Slot)
		}
		t.Fatalf("ReadAll returned %d channels (%v), want 7 — three memories and four call channels", len(cp.Channels), slots)
	}
	populated := 0
	for _, ch := range cp.Channels {
		if ch.Data != nil {
			populated++
		}
	}
	if populated != 7 {
		t.Errorf("%d of the 7 channels carry data, want all 7", populated)
	}
	if cp.Radio.Model != "IC-705" || cp.Radio.CATID != "A4" {
		t.Errorf("Radio = %+v, want the session's own model and address", cp.Radio)
	}
}

func TestDiffAgainstTheSparseBank(t *testing.T) {
	// The addendum's "materialised-set Diff, end to end". A CONSENTED
	// session, so that "permitted" means the entry is not blocked by the
	// write gate either — otherwise every assertion below would pass for
	// the wrong reason.
	r := radioHoldingSlots(t, "G01-001", "G101-001", "G101-002", "G101-003", "G101-004")
	sess := openSession(t, r, WithConsentedUnverifiedWrites())
	caps := sess.Capabilities()

	svc := clone.NewService(sess, clone.SnapshotStore{Dir: t.TempDir()})
	baseline, err := svc.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("clone.ReadAll: %v", err)
	}

	// (a) An ADD at an unmaterialised but IN-SPACE slot is permitted: a
	// sparse bank's Slots lists what a read found, not what the radio can
	// hold.
	added := cloneCodeplug(baseline)
	newData := *baseline.Channels[0].Data
	added.Channels = append(added.Channels, codeplug.Channel{Slot: "G42-007", Data: &newData})
	res, err := codeplug.Diff(baseline, added, caps)
	if err != nil {
		t.Fatalf("Diff with an in-space add: %v", err)
	}
	var addEntry *codeplug.DiffEntry
	for i := range res.Entries {
		if res.Entries[i].Slot == "G42-007" {
			addEntry = &res.Entries[i]
		}
	}
	if addEntry == nil {
		t.Fatal("Diff reported no entry for the added slot G42-007")
	}
	if addEntry.Kind != codeplug.DiffAdded || addEntry.Bank != spec.BankMemory {
		t.Errorf("G42-007 diffed as %v in bank %q, want an add in MEM", addEntry.Kind, addEntry.Bank)
	}
	if addEntry.Blocked {
		t.Errorf("the add is blocked: %s", addEntry.BlockReason)
	}

	// (b) An add OUTSIDE every bank's space is refused outright: G101-005
	// is past the CALL bank's four slots and outside MEM's hundred groups.
	outside := cloneCodeplug(baseline)
	outside.Channels = append(outside.Channels, codeplug.Channel{Slot: "G101-005", Data: &newData})
	if _, err := codeplug.Diff(baseline, outside, caps); err == nil {
		t.Error("Diff accepted an add at G101-005, which no bank of this radio can hold")
	}

	// (c) OVER BUDGET IS REFUSED AT DIFF TIME, with the budget named, and
	// nothing is ever built or sent — what an over-budget IC-705 actually
	// does is undocumented (ic705-group-budget, lift L-OVERBUDGET).
	over := cloneCodeplug(baseline)
	setsBefore := r.Sets()
	for i := 0; i < 501; i++ {
		d := newData
		over.Channels = append(over.Channels, codeplug.Channel{
			Slot: spec.SparseSlot(20+i/100, 1+i%100),
			Data: &d,
		})
	}
	_, err = codeplug.Diff(baseline, over, caps)
	if err == nil {
		t.Fatal("Diff accepted a candidate holding more populated memories than this radio's budget")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("the budget refusal %q does not name the limit", err)
	}
	if r.Sets() != setsBefore {
		t.Error("an over-budget diff put a memory set on the wire")
	}

	// (d) A Modify of a materialised slot diffs normally.
	modified := cloneCodeplug(baseline)
	for i := range modified.Channels {
		if modified.Channels[i].Slot == "G01-001" {
			d := *modified.Channels[i].Data
			d.Tag = "RENAMED"
			modified.Channels[i].Data = &d
		}
	}
	res, err = codeplug.Diff(baseline, modified, caps)
	if err != nil {
		t.Fatalf("Diff with a modify: %v", err)
	}
	if res.Modified != 1 {
		t.Errorf("Diff reported %d modified slots, want 1", res.Modified)
	}
	for _, e := range res.Entries {
		if e.Slot == "G01-001" && e.Blocked {
			t.Errorf("the modify is blocked: %s", e.BlockReason)
		}
	}
}

// cloneCodeplug copies cp deeply enough for a diff fixture: fresh channel
// and data values, so a test mutating one candidate cannot reach the
// baseline it is being compared against.
func cloneCodeplug(cp *codeplug.Codeplug) *codeplug.Codeplug {
	out := *cp
	out.Channels = make([]codeplug.Channel, len(cp.Channels))
	for i, ch := range cp.Channels {
		out.Channels[i] = ch
		if ch.Data != nil {
			d := *ch.Data
			out.Channels[i].Data = &d
		}
	}
	return &out
}
