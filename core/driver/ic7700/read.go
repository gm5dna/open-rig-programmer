// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// slotToAddress maps a canonical wire-form slot to the channel address the
// codec addresses it by, and to the bank it belongs to.
//
// "001".."099" are the memories; "P1" and "P2" are the scan edges. Nothing
// else is a slot on this radio: "000" (there is no channel zero), "100"
// (the memories stop at 99), "P0"/"P3", a bare "1", "" and the sparse
// group form "G05-012" (which belongs to the group-addressed models, not
// to this flat one) are all errors.
//
// MATRIX §1 row 5: P1 AND P2 ARE NOT A SEPARATE BANK IN THE WIRE
// PROTOCOL. They are two more values of the same two-byte selector — PDF
// p.213 (folio 14-13), field q,w, prints "0001-0099: Memory channel 1 to
// 99", "0100: Programmed scan edge P1", "0101: Programmed scan edge P2",
// one contiguous space with three printed forms, corroborated by command
// 08 at PDF p.203 (folio 14-3) — and core/civ/ic7700's profile declares
// exactly that range (ChannelLo 1, ChannelHi 101).
//
// This project models them as a SCAN bank only because the neutral memory
// model needs the distinction between a memory and a scan edge; the codec
// knows nothing of it.
func slotToAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	return icom.SlotToAddress(params.Name, slot)
}

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
// codeplug.Channel.
//
// AN EMPTY SLOT COMES BACK AS AN EMPTY CHANNEL (Data nil), never an error
// that would abort a caller's ReadAll — the neutral contract at
// core/driver/driver.go. Both of this model's two unverified empty
// readings land there: a rejected read (T4) and an all-0xFF record (see
// recordIsAbsent).
//
// A WRONG RECORD LENGTH IS AN ERROR, and deliberately not an empty
// channel: no partial parse, no fake Unavailable channel (spec D4,
// adjudication 13).
//
// NEVER A GUESSED VALUE ANYWHERE. Every field the 1A 00 record does not
// express comes back Unavailable — "there is no such field" — rather than
// Unknown, which would mean "the radio has one and this read did not learn
// it". The two E6-unmapped nibbles come back Unavailable too: an unmapped
// region is not decoded, so there is nothing to report.
//
// THE TONE ARMS ARE TIER RULING T1(3). A civ-layer tone number INSIDE the
// declared domain maps to a Known ToneField; one OUTSIDE it — 0 INCLUDED —
// maps to Unknown. The civ layer is lossless and semantics-free (T1(1)):
// it hands up the number 0 unharmed from a tone-OFF channel whose bytes
// are 00 00 00. The CAPABILITY does not admit 0, because 0 Hz is not a
// tone (T1(2)). So the DRIVER is where the difference is resolved, and it
// resolves it towards Unknown: A READ NEVER CONSTRUCTS A KNOWN VALUE
// codeplug.Validate WOULD THEN REFUSE.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	return icom.ReadChannel(ctx, &params, s.eng, s.caps, &s.answerMismatches, slot)
}
