// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7610

import "time"

// defaultIDToken is what a 19 00 request answers with when WithIDToken has not
// been used.
//
// IT IS INVENTED. IT LIFTS NOTHING. The IC-7610 CI-V Reference Guide prints the
// 19 00 command and prints no reply value for it, anywhere — not in the command
// table, not in the worked example, not in any diagram. There is therefore no
// printed value to transcribe and no captured value to copy, and the width of
// the field is as undocumented as its contents.
//
// 0xA5 was chosen BECAUSE it is obviously synthetic: an alternating bit
// pattern, and not one of the addresses or codes this package names (0x98,
// 0xE0, 0x00, 0xFB, 0xFA, 0xFD, 0xFE). A plausible-looking default would be
// worse than an implausible one — a consumer whose ID probe happens to expect
// the right value would pass against a fake that guessed, and nobody would
// learn that the guess was a guess. This one fails loudly, and WithIDToken is
// how a consumer that knows better says so.
//
// One byte, for the same reason. See PROVENANCE.md.
var defaultIDToken = []byte{0xA5}

// defaultQueuedFrames bounds the radio's output queue. Reached only by a
// consumer that runs a flood and never reads; the oldest frames are then
// dropped rather than the queue growing without limit. See doc.go,
// "Concurrency and the pipe".
const maxQueuedFrames = 4096

// config is what the options build. Options mutate a config, not a Radio, so
// that no option can touch a radio that is already serving its port: every knob
// here is fixed before the first goroutine starts, and the only things that
// change afterwards are the floods, which have their own explicit
// post-construction controls.
type config struct {
	idToken         []byte
	usbEcho         bool
	transceiveEvery time.Duration
	addressedEvery  time.Duration
	recordLen       int
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

// WithIDToken sets the data bytes a 19 00 request is answered with.
//
// This option exists because the reply value is UNDOCUMENTED (see
// defaultIDToken): the guide prints the command and never prints what it
// answers. A consumer that has an actual IC-7610's answer — from hardware, or
// from a capture, neither of which this project has — supplies it here, and
// that is the only way a correct value ever gets into this package. The default
// is invented and should be expected to be wrong.
//
// The token is copied, so a caller may reuse its slice. A token containing 0xFD
// or 0xFE will truncate or resynchronise the frame carrying it; see doc.go,
// "Framing".
func WithIDToken(tok []byte) Option {
	return func(c *config) { c.idToken = append([]byte(nil), tok...) }
}

// WithUSBEcho makes the radio echo every received frame back verbatim, before
// any answer to it.
//
// One option covers two cases the guide describes separately — a "CI-V USB Echo
// Back" setting, and a [REMOTE]-linked bus case — because they look identical
// on the wire, and this package models the wire. A consumer cannot tell them
// apart and neither can a radio's peer.
//
// The echo runs BEFORE the address filter, so a frame addressed to another
// radio is echoed and then ignored. That is a modelling decision about where an
// echo sits, recorded in doc.go and PROVENANCE.md; the document does not say.
func WithUSBEcho() Option {
	return func(c *config) { c.usbEcho = true }
}

// WithTransceiveFlood starts a BROADCAST flood at construction: a frame every
// `every`, addressed to 0x00 — to nobody in particular.
//
// This is transceive traffic. That unsolicited frames carry `to` = 00 is
// ASSUMED, not evidenced: the document prints no broadcast frame at all. The
// option asserts the assumption so a consumer can be tested against it, and
// asserting it is not evidence for it.
//
// It is INDEPENDENT of WithAddressedFlood in every way — separate option,
// separate goroutine, separate stop — because a consumer switches on which of
// the two is running, and merging them into one option with a flag would take
// that distinction away. A non-positive interval starts nothing.
func WithTransceiveFlood(every time.Duration) Option {
	return func(c *config) { c.transceiveEvery = every }
}

// WithAddressedFlood starts a CONTROLLER-ADDRESSED flood at construction: a
// frame every `every`, addressed to 0xE0, as though the radio were answering
// continuously and would not stop.
//
// This is a SYNTHETIC line condition. The document describes no radio doing it,
// and this package does not claim one does. It exists because a consumer that
// must survive a jabbering peer has to be shown surviving one, and because it
// is the condition a broadcast flood is most easily confused with.
//
// Independent of WithTransceiveFlood; see there. A non-positive interval starts
// nothing.
func WithAddressedFlood(every time.Duration) Option {
	return func(c *config) { c.addressedEvery = every }
}

// WithRecordLength makes this radio accept and answer memory records of n bytes
// instead of RecordLen.
//
// RecordLen is DERIVED from an evidence artefact's field widths, not read off a
// page and not confirmed against hardware (doc.go, "Record length"). This
// option is the acknowledgement that a derivation can be wrong: a consumer that
// needs to prove its own length handling, or that learns the real length from a
// radio this project has never had, sets it here rather than editing a constant
// and quietly changing what every other test means.
//
// It PANICS on n < 1. A zero-length record is not a shorter record, it is the
// absence of one, and it would make a set and a read indistinguishable.
func WithRecordLength(n int) Option {
	return func(c *config) {
		if n < 1 {
			panic("fakeic7610: WithRecordLength needs at least one byte — a zero-length record would make a set indistinguishable from a read")
		}
		c.recordLen = n
	}
}

// WithLatency delays each ANSWER by d, modelling a radio that does not reply
// instantly.
//
// It applies to answers only. An echo is not an answer — it is the line
// reflecting what it was given — and a flood frame is not one either, so
// neither is delayed. A consumer testing a timeout wants the answer late and
// the line otherwise normal.
//
// No IC-7610 timing has ever been observed by this project, so there is no
// default and nothing here is a model of a real delay. The wait is
// interruptible: Close never has to wait one out.
func WithLatency(d time.Duration) Option {
	return func(c *config) { c.latency = d }
}
