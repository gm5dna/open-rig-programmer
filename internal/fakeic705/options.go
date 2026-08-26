// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic705

import "time"

// Option configures a Radio at construction. Options run inside New, before any
// goroutine starts, which is why the fields they touch need no lock.
type Option func(*Radio)

// WithLatency makes every reply wait d before its bytes are written.
//
// It is NOT a fault: this package models no faults at all (doc.go, "What this
// fake deliberately does NOT model"). Latency is the knob Close's promptness is
// proven against — a Close during a scripted delay must not have to wait the
// delay out — and the knob a timeout test needs to reach a timeout.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.latency = d }
}

// WithRecord seeds one memory slot with a record.
//
// THE RECORD MAY BE ANY LENGTH AND IS STORED AND SERVED VERBATIM. This is not
// an oversight in the length rule; it is the other side of it. The rule the
// manual states — a `1A 00` set whose record is not RecordLen bytes is refused
// — is a rule about WHAT ARRIVES ON THE WIRE, and this fake enforces it there
// without exception. What an operator puts into the radio's state before any
// wire exists is a different act entirely, and a fake that refused it could not
// be made to serve a wrong-length record at all — so nothing could ever test a
// driver that must recognise one, refuse the session and say why. A 39-byte
// record seeded here is answered as 39 bytes.
//
// The group and channel need only fit the address field. A slot outside the
// ranges the manual states may be seeded, and SlotState will report it, but no
// read can ever reach it: the wire refuses the address before it consults the
// state, which is itself worth being able to demonstrate.
//
// The record is copied.
func WithRecord(group, channel int, record []byte) Option {
	mustBeAddressable(group, channel)
	rec := append([]byte(nil), record...)
	return func(r *Radio) {
		r.slots[Slot{Group: group, Channel: channel}] = rec
	}
}

// WithFactoryImage replaces the radio's whole memory with img — every slot at
// once, which is what an inventory walk needs in front of it.
//
// The image is CLONED, so the caller may keep and reuse it. A nil image, and an
// image with no slots, both mean an empty radio.
//
// It REPLACES rather than merges, so order matters when it is combined with
// WithRecord: put the image first and the individual records after it, and the
// records win. The reverse order silently discards them, which is why this is
// said here rather than left to be discovered.
func WithFactoryImage(img Image) Option {
	var clone Image
	if img == nil {
		clone = EmptyImage()
	} else {
		clone = img.Clone()
	}
	return func(r *Radio) { r.slots = clone }
}

// WithNeverQuiet makes the radio emit unsolicited BROADCAST frames — addressed
// to 00, the "to anyone" address — continuously, from construction until Close.
//
// BROADCASTS AND THE ADDRESSED FLOOD ARE NOT INTERCHANGEABLE, and choosing the
// wrong one makes a test prove nothing. A broadcast is not addressed to the
// controller, so a well-built adapter drops it at the framing seam and no
// engine above ever sees it: this option tests THAT DROP, and a consumer whose
// drain never fills under it is behaving correctly. To reach a drain and a cap,
// use WithAddressedFlood, whose frames are addressed to the controller and so
// cannot be dropped on address alone.
//
// "Continuously" is paced by the reader: the pipe is unbuffered, so a frame is
// only written when something reads it. A host that reads nothing sees one
// frame's worth of pressure, not an unbounded queue.
func WithNeverQuiet() Option {
	return func(r *Radio) {
		r.emitters = append(r.emitters, emitter{to: broadcastAddress})
	}
}

// WithBroadcastEvery makes the radio emit one unsolicited BROADCAST frame every
// d — the same frames WithNeverQuiet sends, at a pace a test can reason about.
//
// A d of zero or less is the continuous case and is exactly WithNeverQuiet.
func WithBroadcastEvery(d time.Duration) Option {
	return func(r *Radio) {
		r.emitters = append(r.emitters, emitter{to: broadcastAddress, every: d})
	}
}

// WithAddressedFlood makes the radio emit a continuous stream of unsolicited
// frames ADDRESSED TO THE CONTROLLER (E0) — the only unsolicited traffic that
// reaches a consumer's drain, because address alone cannot rule it out.
//
// This is the option that trips a drain cap. See WithNeverQuiet for why the two
// are separate options rather than one with a flag.
func WithAddressedFlood() Option {
	return func(r *Radio) {
		r.emitters = append(r.emitters, emitter{to: controllerAddress})
	}
}
