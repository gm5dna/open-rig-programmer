// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx9000

import "time"

// Option configures a *Radio at construction time. See New.
//
// NO WithModelName: this radio has one row (doc.go), so there is no model to
// select. NO FAULT INJECTION: those exercise core/transport.Engine, a
// model-independent implementation already covered by internal/fakeradio's
// fault suite. WithLatency stays because it is not a fault — it is the knob
// Close's promptness is proven against.
type Option func(*Radio)

// WithLatency makes every reply the fake sends wait d before being written to
// the port — a per-reply delay, applied once per exchange. The wait is
// interruptible: a Close during it abandons the reply and returns promptly.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.pipe.Latency = d
	}
}

// WithSlot overlays one slot's state onto whatever image is already present.
//
// No validation is applied: the state is stored verbatim, so a test may craft
// a slot whose ANSWER is deliberately malformed — a P7 outside {'0','1'}, a
// mode byte the legend does not list — and drive a real driver's
// parse-error path through a real fake rather than through a scripted
// transcript.
func WithSlot(slot string, s MemState) Option {
	return func(r *Radio) {
		r.slots[slot] = s
	}
}

// WithFactoryImage REPLACES the fake's entire slot map with img's output.
// Pass it BEFORE any WithSlot option in the same New call, or the image will
// overwrite it. Without this option, New defaults to DefaultImage.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) {
		r.slots = img()
	}
}

// The three legal CAT-ID answers this radio's own ID legend prints (matrix
// §1.2): the three sub-variant names one physical command set answers under.
const (
	CATID9000D       = "0101"
	CATID9000Contest = "0102"
	CATID9000MP      = "0103"
)

// WithCATID overrides "ID;"'s answer to one of the three values this radio's
// own ID legend prints (register entry 10). It panics on any other value:
// every call is a compile-time-known fixture constant, so an unlisted ID is a
// programming error in the caller, not a wire condition to model.
func WithCATID(id string) Option {
	switch id {
	case CATID9000D, CATID9000Contest, CATID9000MP:
	default:
		panic("fakeftdx9000: WithCATID given an ID this radio's manual does not list: " + id)
	}
	return func(r *Radio) {
		r.catID = id
	}
}
