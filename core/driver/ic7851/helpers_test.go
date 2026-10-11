// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// probeSlotCount is the size of Open's occupied-slot search, written out
// here as the oracle for params.ProbeSlots.
const probeSlotCount = 10

func addressToSlot(a civ.ChannelAddress) (string, error) {
	return icom.AddressToSlot(params.Name, a, "%03d")
}

func recordIsAbsent(raw []byte) bool { return icom.RecordIsAbsent(raw) }

// slotToAddress maps a canonical wire-form slot to the channel address the
// codec addresses it by, and to the bank it belongs to.
//
// "001".."099" are the memories; "P1" and "P2" are the scan edges. Nothing
// else is a slot on this radio: "000" (there is no channel zero), "100"
// (the memories stop at 99), "P0"/"P3", a bare "1", "" and the sparse
// group form "G05-012" (which belongs to the group-addressed models, not
// to this flat one) are all errors.
//
// MATRIX §3.15(d): P1 AND P2 ARE NOT A SEPARATE BANK IN THE WIRE
// PROTOCOL. They are two more values of the same two-byte selector — PDF
// p.263 (folio 18-14), field ①,②, prints "0001–0099: Memory channel 1 to
// 99", "0100: Programmed scan edge P1", "0101: Programmed scan edge P2",
// one contiguous space with three printed forms, corroborated by command
// 08 at PDF p.252 (folio 18-3) — and core/civ/ic7851's profile declares
// exactly that range (ChannelLo 1, ChannelHi 101).
//
// This project models them as a SCAN bank only because the neutral memory
// model needs the distinction between a memory and a scan edge; the codec
// knows nothing of it.
func slotToAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	return icom.SlotToAddress(params.Name, slot)
}

// memorySetStep, requestedFields and the others below are the engine's
// write-path pieces under the names the existing tests use.
const memorySetStep = icom.MemorySetStep

var conditionalRequestedFields = icom.ConditionalRequestedFields

func requestedFields(d codeplug.ChannelData) []spec.Field { return icom.RequestedFields(&params, d) }

func outsideVocabulary(d codeplug.ChannelData, caps spec.Capabilities) (spec.Field, string) {
	return icom.OutsideVocabulary(&params, d, caps)
}

func unmappedRegionsDiffer(raw []byte) error { return icom.UnmappedRegionsDiffer(&params, raw) }

func toneRefusalReason(create bool) string { return icom.ToneRefusalReason(&params, create) }
