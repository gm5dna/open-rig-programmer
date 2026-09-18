// SPDX-License-Identifier: GPL-3.0-or-later

package fakets2000

import (
	"fmt"
	"time"
)

// Option configures a *Radio at construction time. See New.
//
// THE SET IS SMALL, AND IT IS NOT internal/fakeradio's FAULT SUITE. The
// general transport faults exercise core/transport.Engine, one
// model-independent implementation already covered elsewhere, and nothing is
// learnt by running them past a third Kenwood dialect. What IS here is what
// THIS BOOK describes: the two serial-line error tokens (ts2000:9614-9618),
// the transient "?;" suppression the error table itself prints
// (ts2000:9608-9610), and the model label (WithModelName) — doc.go's
// register entry 16.
type Option func(*Radio)

// WithModelName sets the row this Radio reports through Model(). It changes
// NO wire behaviour: doc.go's register entry 16 records that the matrix
// found no per-row difference anywhere in the command tables, and ID answers
// "019" regardless. Without this option, New's model is "TS-2000".
func WithModelName(name string) Option {
	return func(r *Radio) { r.model = name }
}

// WithLatency makes every reply the fake sends wait d before being written to
// the port — a per-reply delay, applied once per exchange. The wait is
// interruptible: a Close during it abandons the reply and returns promptly.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.pipe.Latency = d
	}
}

// WithFactoryImage REPLACES the fake's entire record map with img's output.
// Pass it BEFORE any WithChannel, WithSplitChannel or WithEmptyChannel option
// in the same New call, or the image will overwrite them. Without this
// option, New defaults to DefaultImage.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) {
		r.records = img()
	}
}

// WithChannel overlays ONE half of one channel — the P1='0' record, which is
// an ordinary channel's receive frequency or a section-defined channel's
// start frequency (ts2000:10726-10727). Use WithSplitChannel for the pair.
//
// No validation is applied: the record is stored verbatim, so a test may
// craft a channel whose ANSWER is deliberately malformed and drive a real
// driver's parse-error path through a real fake rather than a scripted
// transcript.
//
// Overlay semantics: it is applied to whatever record map is already
// present.
func WithChannel(channel int, s MemState) Option {
	return func(r *Radio) {
		r.records[recordKey{channel: channel, half: HalfRXOrStart}] = s
	}
}

// WithSplitChannel overlays BOTH halves of one channel: the P1='0' record and
// the P1='1' one. On channels 290-299 the same pair is the section's start
// and end frequency (ts2000:10726-10727); on every other channel it is the
// receive and transmit half.
//
// No validation, and overlay semantics, exactly as WithChannel.
func WithSplitChannel(channel int, rxOrStart, txOrEnd MemState) Option {
	return func(r *Radio) {
		r.records[recordKey{channel: channel, half: HalfRXOrStart}] = rxOrStart
		r.records[recordKey{channel: channel, half: HalfTXOrEnd}] = txOrEnd
	}
}

// WithEmptyChannel removes BOTH halves of channel from the record map, so a
// subsequent MR of either half answers this fake's invented empty-channel
// shape (doc.go's register entry 3 — CANNOT-ESTABLISH on this row, unlike the
// 590 pair's documented sentence). This is the "option where the manual
// leaves it open" the brief asks for: the whole of the empty-channel
// assumption is overridable this way, or wholesale via WithFactoryImage.
//
// Overlay semantics: it is applied to whatever record map is already
// present, so it must be given AFTER any WithFactoryImage in the same New
// call.
func WithEmptyChannel(channel int) Option {
	return func(r *Radio) {
		delete(r.records, recordKey{channel: channel, half: HalfRXOrStart})
		delete(r.records, recordKey{channel: channel, half: HalfTXOrEnd})
	}
}

// WithMemoryReadUnsupported makes an MR of ANY channel answer "?;" while MW
// and MC are untouched — the book's second "?;" cause, played (ts2000:9603-
// 9606: "Command was not executed due to the current status of the
// transceiver"), not a claim that any TS-2000 refuses MR.
func WithMemoryReadUnsupported() Option {
	return func(r *Radio) {
		r.memoryReadUnsupported = true
	}
}

// WithTransientNAKSuppressed makes the fake DROP every "?;" it would
// otherwise send, answering nothing at all instead — the book's own printed
// note under the "?;" row (ts2000:9608-9610).
func WithTransientNAKSuppressed() Option {
	return func(r *Radio) {
		r.transientNAKSuppressed = true
	}
}

// WithSatelliteLiveState seeds the fake's live satellite radio state —
// SA's P1 (satellite mode)/P4 (CTRL main/sub)/P7 (MULTI/CH mode),
// fakets2000.go's satMode/satCtrl/satMulti — away from New's all-OFF
// construction default. It exists so a test can drive that state
// somewhere the driver's own write path (the only other writer of these
// three fields, parser.go's handleSASet) could not have put it, and then
// prove a subsequent channel write threads the SEEDED values through
// unchanged rather than merely echoing back whatever it just wrote itself.
func WithSatelliteLiveState(satModeOn, ctrlOnSub, multiCHMemoryMode bool) Option {
	return func(r *Radio) {
		r.satMode = boolByte(satModeOn)
		r.satCtrl = boolByte(ctrlOnSub)
		r.satMulti = boolByte(multiCHMemoryMode)
	}
}

// WithStreamError scripts one of the two SERIAL-LINE error tokens
// (ts2000:9614-9618) in place of exchange n's reply, where n counts EVENTS
// the fake has handled from 1.
//
// The kind is required: StreamErrorUnset panics rather than defaulting to
// one of the two tokens, which would put a test on the wrong sentence of the
// book. n below 1 panics for the same reason — there is no exchange 0.
func WithStreamError(kind StreamError, n int) Option {
	if kind != StreamErrorE && kind != StreamErrorO {
		panic(fmt.Sprintf("fakets2000: WithStreamError(%v) — the token is REQUIRED; the two are printed with different causes (ts2000:9614-9618)", kind))
	}
	if n < 1 {
		panic(fmt.Sprintf("fakets2000: WithStreamError exchange %d — exchanges are counted from 1", n))
	}
	return func(r *Radio) {
		r.streamErrors[n] = kind
	}
}
