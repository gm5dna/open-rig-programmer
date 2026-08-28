// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mcReadSpec is the transport spec for an MC read: fixed 6-byte answer
// ("MC" + 3-byte slot + ";", the same shape as an MC-set frame). One
// retry — reads are idempotent.
func mcReadSpec() transport.CommandSpec {
	return transport.CATReadSpec("MC", 6, 1)
}

// ErrMCSnapshotUnavailable is the sentinel a caller should compare
// against (via errors.Is) when CurrentMemory cannot report a usable
// snapshot: a transport failure, or an MC answer this codec cannot parse
// (most notably "000" — the VFO/no-stored-memory case whose semantics
// remain UNTESTED, core/cat/mc.go's doc comment). The error actually
// returned wraps this sentinel alongside the underlying cause.
var ErrMCSnapshotUnavailable = errors.New("ft710: current memory selection unavailable (VFO state semantics untested — never guess a recall target)")

// CurrentMemory queries the radio's current memory selection (MC;) and
// returns it as a canonical wire-form slot, e.g. "006", "P1L". It always
// reflects the radio's ACTUAL current selection, however it got there —
// an MC-set recall, front-panel operation, or an MW write, which
// HW-CONFIRMED 2026-07-13 (M5b write trials, docs/hardware-notes.md)
// moves the radio's selection to the written slot too, hands-off.
//
// This is deliberately NOT part of driver.Session: it is a minimal,
// additive surface on the concrete *Session (controller design decision,
// task 20 brief) that only core/clone's Execute uses, via a small
// optional interface (core/clone.MemorySelector) and a type assertion —
// so driver.Session itself stays unchanged, and fakeradio-driven tests
// exercise this through the REAL driver, never a mock.
//
// If the answer is "000" (VFO/no-stored-memory — never directly observed
// this project's hardware sessions; core/cat/mc.go's doc comment) or
// otherwise fails to parse, CurrentMemory returns an error satisfying
// errors.Is(err, ErrMCSnapshotUnavailable) rather than guessing: the
// caller (clone's Execute) must skip the restore step for this session,
// never invent a recall target.
func (s *Session) CurrentMemory(ctx context.Context) (string, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	frame, err := s.eng.Do(ctx, s.dialect.BuildMCRead(), mcReadSpec())
	if err != nil {
		return "", fmt.Errorf("ft710: CurrentMemory: %w: %w", ErrMCSnapshotUnavailable, err)
	}
	slot, err := s.dialect.ParseMCAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("ft710: CurrentMemory: %w: %w", ErrMCSnapshotUnavailable, err)
	}
	return slot.Wire(), nil
}

// RecallMemory issues an MC-set (recall) for slot: fire-and-forget, with
// the transport's bounded listen for a delayed "?;" rejection, exactly
// like WriteChannel's own MW/MT sends. Reference: "MC — MEMORY CHANNEL
// (recall) ... side effect: recalls the channel on the radio (changes
// operating state!)". Recalling an unpopulated slot is REJECTED —
// ASSUMED, by analogy with MR's identical empty-slot rule; MC-set of an
// empty slot was not itself hardware-probed at M5b (docs/hardware-notes.md's
// "explicitly not probed" list).
func (s *Session) RecallMemory(ctx context.Context, slot string) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return fmt.Errorf("ft710: RecallMemory: %w", err)
	}
	cmd, err := s.dialect.BuildMCSet(sl)
	if err != nil {
		return fmt.Errorf("ft710: RecallMemory: %w", err)
	}
	if _, err := s.eng.Do(ctx, cmd, fnfSpec()); err != nil {
		return fmt.Errorf("ft710: RecallMemory %s: %w", sl.Wire(), err)
	}
	return nil
}
