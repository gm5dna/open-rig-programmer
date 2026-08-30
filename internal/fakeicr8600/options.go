// SPDX-License-Identifier: GPL-3.0-or-later

package fakeicr8600

import "time"

// Option configures a *Radio at construction time. See New.
//
// Options are applied in the order given, over whatever is already there, so a
// later one wins: WithEmpty(0, 3) after WithRecord(0, 3, …) leaves channel 3
// unoccupied, and the two written the other way round leave it occupied.
//
// EVERY OPTION HERE EXISTS BECAUSE THE DOCUMENT LEAVES SOMETHING OPEN. Each one
// names its register entry in doc.go; none of them makes the fake behave in two
// ways for the sake of it.
type Option func(*Radio)

// WithRadioAddress moves the receiver off its default 96h.
//
// The default IS printed — PDF page 3 (folio 2) labels the byte "Receiver's
// default address" in both frame diagrams — and so is the fact that it can be
// changed: "To control the receiver, first set its address, data communication
// speed, and transceive function. These settings are set in Set mode." What is
// NOT printed is the admissible range, or that a moved receiver answers only on
// its new address, which is what this Option lets a consumer exercise. doc.go
// register entry 13.
//
// This program ships no --civ-address flag, so a moved receiver is unreachable
// by it and simply times out — a fake at a moved address is how that is made
// testable rather than merely asserted.
func WithRadioAddress(addr byte) Option {
	return func(r *Radio) { r.radioAddr = addr }
}

// WithIDToken sets the data bytes the fake returns to 19 00.
//
// The DEFAULT is a fixed arbitrary token (defaultIdentityToken), because the
// reply VALUE is printed nowhere and a fake that implied one would be asserting
// a fact nobody has. This Option exists so a consumer can pin a DIFFERENT token
// and prove its driver RECORDS whatever it gets rather than matching a value.
//
// The bytes are copied, and an empty token is honoured as an empty token: the
// fake then answers FE FE E0 96 19 00 FD, which is a legitimate thing for a
// consumer to want to see its driver cope with. doc.go register entry 1.
func WithIDToken(data []byte) Option {
	return func(r *Radio) {
		tok := make([]byte, len(data))
		copy(tok, data)
		r.identityToken = tok
	}
}

// WithRecord seeds one channel with RAW record bytes of ANY length — the fake
// stores what it is given without checking it.
//
// This is the arbitrary-length ability a fingerprint or wrong-sibling test
// needs: a 64-byte record, a 42-byte one, an all-FF one that no IC-R8600 layout
// admits. The refusal rules apply to what ARRIVES ON THE WIRE, never to what a
// consumer seeds.
//
// group and channel are the two printed two-byte address fields, as numbers,
// 0-based: group 0-99 are the Normal memory channel groups, 100 the Auto Write
// Memory group, 101 Scan Skip and 102 Programmable Scan Edge. Either outside
// 0-9999 panics (see bcd2).
//
// The bytes are copied. A record containing FE or FD is stored and returned
// exactly as given, unescaped, which will break the framing of the answer that
// carries it — deliberately: a consumer that seeds such a record is seeding one
// no receiver could send.
func WithRecord(group, channel int, record []byte) Option {
	return func(r *Radio) {
		rec := make([]byte, len(record))
		copy(rec, record)
		r.records[addrOf(group, channel)] = MemState{Record: rec}
	}
}

// WithEmpty marks a channel unoccupied, so a read of it takes the empty-channel
// path (NG by default; see WithEmptyReplyAllFF).
func WithEmpty(group, channel int) Option {
	return func(r *Radio) {
		delete(r.records, addrOf(group, channel))
	}
}

// WithEmptyReplyAllFF makes a read of an unoccupied channel answer a record
// full of FF instead of NG.
//
// THE TWO ANSWERS ARE TWO SEPARATE ASSUMPTIONS, and this Option exists because
// they are graded apart: no single capture can establish both. doc.go register
// entries 5 (the NG answer, the default) and 6 (the all-FF record). A driver
// must cope with either, and only a fake that can be either lets it be shown to.
//
// The record is emptyReplyLen bytes long — the shortest accepted length, which
// invents the least. Its mode byte is FF, which the printed table lists
// nowhere, so the answer deliberately selects no layout: that is the whole
// point of deciding emptiness on raw bytes before any layout is applied.
func WithEmptyReplyAllFF() Option {
	return func(r *Radio) { r.emptyReplyAllFF = true }
}

// WithEcho turns the bus echo on or off; it is off unless this option says
// otherwise.
//
// Two per-port echo-back settings are printed (PDF page 7, folio 6: "1A 05
// 0094" for the front USB port and "1A 05 0096" for the rear), and NEITHER
// DEFAULT IS PRINTED. Off is this package's choice. doc.go register entry 14.
//
// When on, every complete frame is echoed VERBATIM before any answer, including
// one addressed to some other radio: a bus echo is a property of the wire, not
// of who was being spoken to. That is what makes byte-identity echo suppression
// testable — a consumer's suppression must key on the bytes it recorded
// sending, never on position or count.
func WithEcho(on bool) Option {
	return func(r *Radio) { r.echo = on }
}

// WithTransceiveBroadcasts makes the fake emit unsolicited frames addressed to
// the broadcast byte every period, FOREVER, REGARDLESS of whether a request is
// pending — a receiver that never goes quiet.
//
// Transceive exists and is settable ("1A 05 0092 … 00=OFF, 01=ON", PDF page 7),
// and its factory default is printed nowhere. THE BROADCAST FORM ITSELF IS
// ALSO ASSUMED: the guide draws two `to` values, 96 and E0, and no broadcast
// frame anywhere. doc.go register entries 3 and 15.
//
// The frame emitted is a fixed, deliberately meaningless one — this package
// makes no claim about what a real IC-R8600 broadcasts, only about the `to`
// byte a consumer's address filter must survive.
//
// A non-positive period disables the flood.
func WithTransceiveBroadcasts(period time.Duration) Option {
	return func(r *Radio) { r.broadcastPeriod = period }
}

// WithLatency makes every reply the fake sends wait d before being written to
// the port.
//
// THE WAIT DOES NOT BLOCK THE FAKE. The delay is scheduled, not slept through
// in the serve loop, so a scripted latency holds up the reply and NOTHING ELSE:
// the broadcast flood keeps emitting throughout it, further requests keep being
// read, and Close returns promptly rather than waiting the delay out. That is
// the difference between "a receiver that is slow to answer" and "a receiver
// that has stopped".
//
// Replies are still written in the order they were produced.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.latency = d }
}

// WithShortSetsAccepted makes the fake accept a HEAD-ONLY set for a mode class
// whose layout has a tail, filling the omitted tail bytes with fill.
//
// The document is unusually generous here, and unusually vague. The note
// beneath the record diagram on PDF page 12 reads, in full: "In the modes other
// than FM and Digital, (42) and or later is not used. In the FM and Digital
// modes, entering (42) and or later can be omitted. The default value is
// applied to the omitted items." So a short set IS accepted — that much is
// printed — and WHAT THE DEFAULTS ARE is printed nowhere, for any of the
// omitted bytes, in any tail.
//
// DEFAULT: REFUSED. A fake that filled the omitted bytes in on its own
// authority would be inventing up to eight values and then serving them back as
// though a receiver had supplied them. Refusing invents nothing, and this
// program always sends the full layout for the mode in any case.
//
// This Option exists because the refusal is the FAKE's choice and not the
// radio's behaviour, and the open point must stay exercisable. The fill byte is
// the CALLER's, deliberately: this package will not choose one. doc.go register
// entry 11.
func WithShortSetsAccepted(fill byte) Option {
	return func(r *Radio) {
		r.shortSets = true
		r.shortSetFill = fill
	}
}

// broadcastFrame is the unsolicited frame WithTransceiveBroadcasts emits:
// FE FE 00 <radio> 00 <five bytes> FD, command 00 being the "Output the
// frequency data for transceive" row of the command table (PDF page 4).
//
// Its DATA is meaningless and says so: five zero bytes where a frequency would
// sit. The claim this frame makes is about its `to` byte and nothing else.
func (r *Radio) broadcastFrame() []byte {
	out := []byte{preambleByte, preambleByte, broadcastAddr, r.radioAddr, 0x00}
	out = append(out, 0x00, 0x00, 0x00, 0x00, 0x00)
	return append(out, endOfMessage)
}
