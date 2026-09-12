// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7600

import "time"

// defaultIDToken is what a 19 00 request answers with when WithIDToken has
// not been used.
//
// IT IS INVENTED. IT LIFTS NOTHING (doc.go, register FAKE-1). The matrix
// prints the 19 00 command and prints no reply value for it anywhere
// (§3.12(i)). 0xA6 was chosen because it is obviously synthetic and is not
// one of the addresses or codes this package names (0x7A, 0xE0, 0x00,
// 0xFB, 0xFA, 0xFD, 0xFE). A plausible-looking default would be worse than
// an implausible one: a consumer whose ID probe happened to expect the
// right value would pass against a fake that guessed, and nobody would
// learn the guess was a guess.
var defaultIDToken = []byte{0xA6}

// maxQueuedFrames bounds the radio's output queue.
const maxQueuedFrames = 4096

// config is what the options build. Options mutate a config, not a Radio, so
// that no option can touch a radio that is already serving its port.
type config struct {
	idToken         []byte
	transceiveEvery time.Duration
	addressedEvery  time.Duration
	recordLen       int
	allFFEmpty      bool
	shortSetPad     bool
	latency         time.Duration
}

func defaultConfig() config {
	return config{
		idToken:   append([]byte(nil), defaultIDToken...),
		recordLen: RecordLen,
	}
}

// Option configures a Radio at construction.
type Option func(*config)

// WithIDToken sets the data bytes a 19 00 request is answered with. See
// defaultIDToken: the reply value is UNDOCUMENTED, and this is the only way
// a correct one ever gets into this package.
func WithIDToken(tok []byte) Option {
	return func(c *config) { c.idToken = append([]byte(nil), tok...) }
}

// WithTransceiveFlood starts a BROADCAST flood at construction: a frame
// every `every`, addressed to 0x00 — ASSUMED, matrix §3.5(b). A
// non-positive interval starts nothing.
func WithTransceiveFlood(every time.Duration) Option {
	return func(c *config) { c.transceiveEvery = every }
}

// WithAddressedFlood starts a CONTROLLER-ADDRESSED flood at construction: a
// frame every `every`, addressed to 0xE0 — a SYNTHETIC line condition the
// matrix's document describes no radio producing. A non-positive interval
// starts nothing.
func WithAddressedFlood(every time.Duration) Option {
	return func(c *config) { c.addressedEvery = every }
}

// WithRecordLength makes this radio accept and answer memory records of n
// bytes instead of RecordLen. RecordLen is DERIVED, not printed (doc.go,
// "Record length"); a consumer that needs to prove its own length handling
// sets it here rather than editing a constant.
//
// PANICS on n < 1.
func WithRecordLength(n int) Option {
	return func(c *config) {
		if n < 1 {
			panic("fakeic7600: WithRecordLength needs at least one byte — a zero-length record would make a set indistinguishable from a read")
		}
		c.recordLen = n
	}
}

// WithAllFFEmpty switches an unset channel's read from CodeNG to an
// all-0xFF record of RecordLen bytes. Matrix §3.8(a) grades the default
// (NG) as ASSUMED from a single capture; §3.8(b) grades the alternative —
// that a record which reads back as all-FF means "empty" — as equally
// undocumented. This package does not pick a winner; it offers both. See
// doc.go, register FAKE-2.
func WithAllFFEmpty() Option {
	return func(c *config) { c.allFFEmpty = true }
}

// WithShortSetAccepted switches a 1A 00 set carrying FEWER than RecordLen
// bytes from refused (the default) to accepted, storing the record
// zero-padded on the tail out to RecordLen.
//
// Matrix §3.10 grades only whether the FULL record is mandatory as ASSUMED
// ("the document never states whether a short record is accepted, padded
// or rejected"); it names no pad convention for the accepted case, because
// the matrix's own default reading is that nothing but a full record is
// accepted at all. The zero-tail padding here is this package's own choice
// — doc.go, register FAKE-3 — not a citation. A set LONGER than RecordLen
// is always refused, in both modes.
func WithShortSetAccepted() Option {
	return func(c *config) { c.shortSetPad = true }
}

// WithLatency delays each ANSWER by d. It applies to answers only — not
// flood frames. No IC-7600 timing has ever been observed by this project
// (matrix §0), so there is no default and nothing here models a real
// delay. The wait is interruptible: Close never has to wait one out.
func WithLatency(d time.Duration) Option {
	return func(c *config) { c.latency = d }
}
