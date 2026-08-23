// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import "fmt"

// THE BUILDER SET IS DELIBERATELY THREE, and what is ABSENT is the point.
//
// There is NO clear/erase builder and NO transceive-set builder in this
// tier (spec D1, adjudications 3 and 19), and no 1A 05 menu surface at
// all. Icom documents a clear form; this program does not build one, its
// gate does not admit one, and every Icom driver reports FieldErase
// unsupported. Nothing may send either form, so the way to guarantee that
// is for neither to exist here — a frame no builder can name is a frame
// AllowedCommand refuses by construction rather than by a rule someone
// could relax.
//
// Every builder takes a Profile receiver, including the one whose frame
// carries no parameters: `19 00`'s `to` and `from` bytes are profile data,
// so there is no radio-independent form of it to return. That is a
// deliberate difference from core/cat, three of whose builders emit fixed
// literals on any receiver — CI-V has no frame at all without an address.

// frameFrom builds FE FE <radio> <controller> body… FD for THIS profile.
//
// It allocates a fresh slice, which is what lets newCommand skip a copy:
// every builder here constructs into a local buffer, never a
// caller-supplied one.
func (p Profile) frameFor(body []byte) []byte {
	out := make([]byte, 0, 5+len(body))
	out = append(out, PreambleByte, PreambleByte, p.radioAddr, p.controllerAddr)
	out = append(out, body...)
	return append(out, EndByte)
}

// BuildTransceiverIDRead builds the `19 00` transceiver-address read.
//
// It is the probe's first frame (spec D3.2): the REPLY VALUE is
// undocumented on all six models in this tier, so it is recorded as a
// diagnostic and never matched — what the probe requires is an
// ADDRESS-MATCHED reply, which is a property of the frame's `to` and
// `from` bytes rather than of its contents.
func (p Profile) BuildTransceiverIDRead() (Command, error) {
	if !p.Configured() {
		return Command{}, fmt.Errorf("civ: unconfigured profile builds nothing")
	}
	return newCommand(p.frameFor([]byte{CmdTransceiverID, SubTransceiverID})), nil
}

// BuildMemoryRead builds the `1A 00 <address>` memory-record read request.
//
// THE REQUEST FORM IS ASSUMED FAMILY-WIDE (spec D5 entry 1): no document
// in this tier prints the read request with no data field, and every
// model's register carries that assumption with its own named lift.
func (p Profile) BuildMemoryRead(addr ChannelAddress) (Command, error) {
	if !p.Configured() {
		return Command{}, fmt.Errorf("civ: unconfigured profile builds nothing")
	}
	a, err := p.encodeAddress(addr)
	if err != nil {
		return Command{}, err
	}
	body := make([]byte, 0, 2+len(a))
	body = append(body, CmdMemory, SubMemoryContents)
	body = append(body, a...)
	return newCommand(p.frameFor(body)), nil
}

// BuildMemorySet builds the `1A 00 <address> <record>` memory-record set:
// the ONE frame this tier writes to a radio outside its consent regime's
// reach, and the only builder here that mutates anything.
//
// It emits the profile's BuildRecordLength, and it validates rec through
// validateRecordFields — the SAME validator AllowedCommand re-runs on the
// frame it decodes — so a record this builder accepts is a frame the gate
// admits, and one it refuses is a frame the gate refuses.
//
// The full record is always sent. Spec D5 entry 4 records that three
// models' documents mark a duplicated TX block "recommended" on write,
// which is advisory prose about a fixed-width field: the driver has no way
// to send half a record, and a layout mapping the block twice is how a
// profile says the copies must agree.
func (p Profile) BuildMemorySet(rec MemoryRecord) (Command, error) {
	if !p.Configured() {
		return Command{}, fmt.Errorf("civ: unconfigured profile builds nothing")
	}
	a, err := p.encodeAddress(rec.Address)
	if err != nil {
		return Command{}, err
	}
	body := make([]byte, 0, 2+len(a)+p.buildLength)
	body = append(body, CmdMemory, SubMemoryContents)
	body = append(body, a...)

	record, err := p.encodeRecord(rec, p.buildLength)
	if err != nil {
		return Command{}, err
	}
	body = append(body, record...)

	frame := p.frameFor(body)
	if len(frame) > p.maxFrame {
		// Unreachable for a profile NewProfile built — V9 proves the
		// arithmetic at construction — and asserted rather than assumed,
		// because the cost of being wrong is a frame this profile's own
		// accumulator would discard as contamination.
		return Command{}, fmt.Errorf("civ: %s: memory set is %d bytes, past this profile's own %d-byte frame bound", p.model, len(frame), p.maxFrame)
	}
	return newCommand(frame), nil
}
