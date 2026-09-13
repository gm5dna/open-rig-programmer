// SPDX-License-Identifier: GPL-3.0-or-later

package fakets870s

import "time"

// Option configures a *Radio at construction time. See New.
//
// THE SET IS SMALL, AND THERE IS NO WithStreamError HERE. doc.go's register
// entry STREAM ERRORS ARE NOT SCRIPTABLE explains why: reviews/driver-ts870s.md's
// `## Verdict` states core/driver/ts870s never builds a live session at all,
// so a scriptable "E;"/"O;" token would have nothing on the driver side to
// exercise.
type Option func(*Radio)

// WithLatency makes every reply the fake sends wait d before being written to
// the port — a per-reply delay, applied once per exchange.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.pipe.Latency = d
	}
}

// WithFactoryImage REPLACES the fake's entire record map with img's output.
// Pass it BEFORE any WithChannel/WithSplitChannel/WithEmptyChannel option in
// the same New call, or the image will overwrite them. Without this option,
// New defaults to DefaultImage.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) {
		r.records = img()
	}
}

// WithChannel overlays ONE half of one channel — the P1='0' record (the
// receive frequency of channels 00-98, or the start frequency of channel
// 99). No validation is applied: the record is stored verbatim, so a test
// may craft a deliberately malformed answer and drive a real driver's
// parse-error path through a real fake.
//
// Overlay semantics: applied to whatever record map is already present.
func WithChannel(channel int, s MemState) Option {
	return func(r *Radio) {
		r.records[recordKey{channel: channel, half: HalfRXOrStart}] = s
	}
}

// WithSplitChannel overlays BOTH halves of one channel at once: rxOrStart at
// P1='0', txOrEnd at P1='1'. Unlike internal/fakets480's row, the TX/End half
// is a published field on this row for channels 00-98 (matrix §2,
// FieldTxFrequency: rw), so a test exercising it needs a way to stage it
// directly rather than only through an MW.
//
// Overlay semantics: applied to whatever record map is already present.
func WithSplitChannel(channel int, rxOrStart, txOrEnd MemState) Option {
	return func(r *Radio) {
		r.records[recordKey{channel: channel, half: HalfRXOrStart}] = rxOrStart
		r.records[recordKey{channel: channel, half: HalfTXOrEnd}] = txOrEnd
	}
}

// WithEmptyChannel removes BOTH halves of channel from the record map, so a
// subsequent MR of either half answers the ZERO record (doc.go's register
// entry 5) — the documented vacant-channel shape (ts870s:9101-9104).
//
// IT INTRODUCES NO NEW ASSUMED BEHAVIOUR: it only removes map entries, which
// triggers the fake's existing zero-record answer.
//
// Overlay semantics: applied to whatever record map is already present, so
// it must be given AFTER any WithFactoryImage in the same New call.
func WithEmptyChannel(channel int) Option {
	return func(r *Radio) {
		delete(r.records, recordKey{channel: channel, half: HalfRXOrStart})
		delete(r.records, recordKey{channel: channel, half: HalfTXOrEnd})
	}
}

// WithMemoryReadUnsupported makes an MR of ANY channel answer "?;" while MW
// and AI are untouched. NOT A CLAIM THAT ANY TS-870S REFUSES MR: it plays
// the error table's second printed cause, "Command was not executed due to
// the current status of the transceiver (even though the command syntax was
// correct)." (ts870s:8434-8438).
func WithMemoryReadUnsupported() Option {
	return func(r *Radio) {
		r.memoryReadUnsupported = true
	}
}

// WithTransientNAKSuppressed makes the fake DROP every "?;" it would
// otherwise send, answering nothing at all instead — the book's own note,
// played: "Occasionally this message may not appear due to microprocessor
// transients in the transceiver." (ts870s:8440-8442). Accepted answers and
// fire-and-forget successes are untouched.
func WithTransientNAKSuppressed() Option {
	return func(r *Radio) {
		r.transientNAKSuppressed = true
	}
}
