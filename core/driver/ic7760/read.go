// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// slotToAddress maps a canonical wire-form slot to the channel address the
// codec addresses it by, and to the bank it belongs to ("001".."099" the
// memories, "P1" and "P2" the scan edges).
//
// MATRIX §3.15(d): P1 AND P2 ARE NOT A SEPARATE BANK IN THE WIRE
// PROTOCOL. They are two more values of the same two-byte selector — PDF
// p.20 (folio 19) prints "00 01 ~ 00 99: Memory channel 01 ~ 99",
// "01 00: Programmed scan edge P1", "01 01: Programmed scan edge P2", one
// contiguous space with three printed forms — and core/civ/ic7760's
// profile declares exactly that space as base MEM 1..99 plus one
// ExtraRange 100..101 (finding F7).
//
// This project models them as a SCAN bank only because the neutral memory
// model needs the distinction between a memory and a scan edge; the codec
// knows nothing of it.
func slotToAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	return icom.SlotToAddress(params.Name, slot)
}

// readRaw is the one read primitive, shared with WriteChannel's
// preservation read.
func (s *Session) readRaw(ctx context.Context, a civ.ChannelAddress) (civ.MemoryRecord, []byte, bool, error) {
	return icom.ReadRaw(ctx, &params, s.eng, &s.answerMismatches, a)
}

// AnswerMismatches reports how many memory answers this session has seen
// whose decoded channel address was not the one requested (tier ruling
// T2). A diagnostic count beside the typed error, so a bus that
// occasionally mis-attributes is visible even when each individual read
// was refused correctly.
func (s *Session) AnswerMismatches() uint64 { return s.answerMismatches.Load() }

// ReadChannel implements driver.Session: ONE 1A 00 read, mapped into one
// codeplug.Channel by the shared engine.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	return icom.ReadChannel(ctx, &params, s.eng, s.caps, &s.answerMismatches, slot)
}
