// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic705

// This file holds everything the fake REMEMBERS, and the accessors a test
// reaches for it through. All of it sits behind Radio.mu, because the servicing
// goroutine writes it while a test goroutine reads it (run with -race).

// SlotState reports the record this fake holds for a slot, and whether it holds
// one at all.
//
// The returned record is a COPY: a caller may keep it, and a later set to the
// same slot will not reach into it.
//
// occupied is the whole of what "written" means here — presence in the map, not
// a flag inside the record and not a content test. A slot holding a zero-length
// record is occupied; a slot never seeded and never set is not. An UNOCCUPIED
// slot returns a nil record, and a caller must read occupied rather than test
// the record for emptiness, since those two states are distinguishable and
// distinct.
func (r *Radio) SlotState(group, channel int) (record []byte, occupied bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.slots[Slot{Group: group, Channel: channel}]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), rec...), true
}

// FramesSeen reports how many frames this fake has parsed since construction.
//
// EVERY frame counts: one addressed elsewhere, one it refused, one it could not
// make sense of at all. The count is of TERMINATED BYTE RUNS the reassembler
// delivered — everything from the start of accumulation up to and including an
// FD — so a run of junk ending in FD counts as one, and a run discarded after
// an accumulator overflow counts as none, because no terminator ever closed it.
//
// It exists so that a test can distinguish "the driver sent nothing" from "the
// driver sent something the fake refused", which are two very different
// findings and produce the same silence on the state.
func (r *Radio) FramesSeen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.framesSeen
}

// SetsSeen reports how many `1A 00` SET frames this fake has been asked to
// take since construction.
//
// It counts ATTEMPTS, not acceptances: a set-shaped frame refused for its
// record length, or for an out-of-range address, counts here just as an
// accepted one does. That is the useful semantic for the question this counter
// exists to answer — "did the write ever leave the driver?" — where zero means
// nothing was attempted and one means something was attempted and, perhaps,
// properly refused.
//
// Set-shaped means a `1A 00` frame, addressed to this radio, whose data area is
// longer than the four address bytes. A `1A 00` carrying exactly the address is
// a read and is not counted; a `1A 00` carrying less than an address is neither
// and is not counted.
func (r *Radio) SetsSeen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.setsSeen
}

// AnswerNextReadWithAddress arms this fake's ONE MISBEHAVIOUR: the next read it
// answers with a record answers it under the address given here, whatever
// address was asked for.
//
// This is deliberate wire misbehaviour, and it is the only kind this package
// models. A driver that trusts a memory answer to be about the channel it asked
// for has a bug that no well-behaved fake can expose; a fake that cannot
// misbehave cannot prove the driver catches misbehaviour.
//
// It is a ONE-SHOT and it is spent only by an ANSWER: a read that draws NG —
// an unwritten slot, an out-of-range address — leaves it armed for the next
// read that does produce a record. A second call before the first is spent
// replaces it. The group and channel need not be in the ranges the manual
// states, and usually should not be; they need only fit the address field
// (mustBeAddressable).
func (r *Radio) AnswerNextReadWithAddress(group, channel int) {
	mustBeAddressable(group, channel)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.wrongAddress = &Slot{Group: group, Channel: channel}
}

// storeRecord writes a record into a slot, replacing whatever was there.
// The record is copied, so the frame buffer it arrived in may be reused.
func (r *Radio) storeRecord(group, channel int, record []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slots[Slot{Group: group, Channel: channel}] = append([]byte(nil), record...)
}

// lookupRecord reads a slot from inside the servicing goroutine. The returned
// slice is the stored one, not a copy: its only caller builds a reply from it
// under no lock and never retains or mutates it.
func (r *Radio) lookupRecord(group, channel int) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.slots[Slot{Group: group, Channel: channel}]
	return rec, ok
}

// takeWrongAddress consumes the armed misbehaviour, if any.
func (r *Radio) takeWrongAddress() (Slot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.wrongAddress == nil {
		return Slot{}, false
	}
	slot := *r.wrongAddress
	r.wrongAddress = nil
	return slot, true
}

func (r *Radio) countFrame() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.framesSeen++
}

func (r *Radio) countSet() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setsSeen++
}
