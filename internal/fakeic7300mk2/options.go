// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300mk2

import (
	"fmt"
	"time"
)

// Option configures a *Radio at construction time. See New.
//
// Options are applied in the order given, before any goroutine starts, so an
// option may rely on seeing whatever an earlier one set. Two of them PANIC on
// a bad argument — WithIDToken on an empty token, WithChannel and
// WithRawChannel on an address that is not one of the three printed forms (and
// WithChannel on a record of the wrong length). A panic is right for these:
// every argument is a compile-time constant of the test that passes it, so a
// bad one is a programming error, and the alternative — a silently unseeded
// slot, or an identity answer with an empty data area — surfaces several
// layers away as "the driver does not recognise this radio".
type Option func(*Radio)

// WithLatency makes every ANSWER the fake sends wait d before it is written to
// the port — a per-exchange delay, applied once per answer. It does not delay
// the echo of WithEcho, which models the bus rather than the radio's thinking
// time, nor the unsolicited frames of WithTransceiveBroadcasts and
// WithAddressedFlood, which have their own periods.
//
// The wait is interruptible: a Close during it abandons the answer and returns
// promptly, so a test may script a multi-second latency without a multi-second
// teardown.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.latency = d }
}

// WithIDToken fixes the data area of the identity answer (19 00). The value is
// undocumented for this radio, so there is nothing for the fake to derive: a
// caller that cares what comes back must say what comes back.
//
// Without this option the answer carries one byte — the radio's own configured
// CI-V address, which is the reading that stays true when WithRadioAddress
// moves it. The token must carry at least one byte, since the answer does;
// WithIDToken panics on an empty one.
func WithIDToken(token []byte) Option {
	return func(r *Radio) {
		if len(token) == 0 {
			panic("fakeic7300mk2: WithIDToken needs at least one byte — the identity answer carries a data area")
		}
		r.idToken = append([]byte(nil), token...)
	}
}

// WithEcho turns on bus echo: every COMPLETE frame the radio sees is written
// straight back to the port before anything else, exactly as a real single-wire
// CI-V bus lets a controller hear its own transmission.
//
// The echo covers frames addressed to somebody else as well, because the bus
// does not read addresses. Echoed frames appear in Sent() along with
// everything else the radio put on the wire.
func WithEcho(on bool) Option {
	return func(r *Radio) { r.echo = on }
}

// WithTransceiveBroadcasts makes the radio emit an unsolicited frequency
// report addressed to 00 — the CI-V broadcast address — every period, forever,
// modelling a radio with its transceive setting on.
//
// It exists so a caller can prove that a continuous stream of traffic which is
// not addressed to the controller does not wedge the radio or the code reading
// it: the answers to real commands must still arrive, in order, amongst the
// broadcasts. A period of zero or less disables the option.
func WithTransceiveBroadcasts(period time.Duration) Option {
	return func(r *Radio) { r.broadcast = period }
}

// WithAddressedFlood makes the radio emit an unsolicited frequency report
// addressed to the CONTROLLER (E0) every period, forever — a radio that is
// never quiet, and whose noise a controller cannot dismiss on the address byte
// alone.
//
// This is the only traffic this fake produces that can reach a reading
// engine's drain cap, and therefore the only way to exercise the path where
// that cap is hit and treated as non-fatal at Init. A period of zero or less
// disables the option.
func WithAddressedFlood(period time.Duration) Option {
	return func(r *Radio) { r.flood = period }
}

// WithRadioAddress moves this radio off its default CI-V address of B6.
//
// IT IS A SIMULATOR CONFIGURATION FOR CONSTRUCTING A SAME-ADDRESS COLLISION IN
// A TEST, AND IS NOT A CLAIM THAT ANY RADIO BEHAVES SO. Pass it E0, the
// controller's own address, and every answer this radio sends has `to` and
// `from` both reading E0 — a frame a controller cannot tell from its own echo.
// That is a fault to be reproduced deliberately, not a mode a real IC-7300MK2
// is being asserted to have.
//
// Every byte of every answer that names the radio follows this: the identity
// answer, the record answer, the OK frame and the NG frame alike carry addr as
// their `from`, and the radio answers only frames whose `to` byte is addr.
//
// addr is not checked against the reserved framing bytes 0xFE (preamble) and
// 0xFD (end-of-message): pass one and this radio emits it verbatim as the
// `from` byte of every frame it sends (civ.go:164), producing malformed
// frames on the wire. This is a test seam, deliberately unvalidated, not a
// guarantee that a real IC-7300MK2 would accept or produce such an address.
func WithRadioAddress(addr byte) Option {
	return func(r *Radio) { r.addr = addr }
}

// WithChannel seeds ONE slot with a record. addr is the canonical slot string
// — "001" … "099", "P1" or "P2" — and record must be exactly the length the
// transcription's field widths sum to, or the option panics.
//
// The record's VALUES are stored verbatim and are not checked against the
// printed vocabularies, deliberately: this is the seam for handing a driver a
// record whose mode byte, or tone byte, or name byte is one the manual never
// prints, and watching what it does with it. A set arriving over the WIRE is
// checked field by field; a seed is not. Use WithRawChannel to bypass the
// length check as well.
func WithChannel(addr string, record []byte) Option {
	return func(r *Radio) {
		slot, ok := canonicalSlot(addr)
		if !ok {
			panic(fmt.Sprintf("fakeic7300mk2: WithChannel(%q): not a channel address — the ①, ② legend prints three forms, \"001\"…\"099\", \"P1\" and \"P2\"", addr))
		}
		if len(record) != recordLen {
			panic(fmt.Sprintf("fakeic7300mk2: WithChannel(%q): record is %d bytes, want %d — use WithRawChannel to seed a foreign length on purpose", addr, len(record), recordLen))
		}
		r.channels[slot] = append([]byte(nil), record...)
	}
}

// WithRawChannel seeds a slot with a record of ANY length, bypassing the
// length check WithChannel applies. A read of that slot then answers the
// record as stored, so a caller can construct an answer of a foreign length
// and drive the reading side's own length check with it.
//
// The address is still checked: an address outside the three printed forms
// could never be read back, since a read carrying one is refused before the
// store is consulted, so seeding it would be a silent no-op. That panics.
func WithRawChannel(addr string, record []byte) Option {
	return func(r *Radio) {
		slot, ok := canonicalSlot(addr)
		if !ok {
			panic(fmt.Sprintf("fakeic7300mk2: WithRawChannel(%q): not a channel address — the ①, ② legend prints three forms, \"001\"…\"099\", \"P1\" and \"P2\"", addr))
		}
		r.channels[slot] = append([]byte(nil), record...)
	}
}
