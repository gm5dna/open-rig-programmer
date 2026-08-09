// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

import "time"

// Option configures a *Radio at construction time. See NewD and NewMP.
//
// THE SET IS SMALL, AND SMALLER THAN fakeradio's ON PURPOSE: there is no fault
// injection here at all. doc.go's "What this fake deliberately does NOT model"
// section names the seven faults fakeradio carries — FaultDropReplies,
// FaultGarbleReply, FaultSpuriousFrame, FaultDelayedRejection,
// FaultDelayedReply, FaultDisconnect, FaultChunkedReplies — and states why none
// is copied (plan decision D6, on internal/fakedx10's recorded reasoning at its
// own options.go:9-16 and doc.go:90-105): in short, they exercise
// core/transport.Engine, which is one model-independent implementation already
// covered by fakeradio's fault suite, and no wiring, CLI or GUI path uses faults
// against a fake rig. WithLatency stays because it is NOT a fault: it is the
// knob Close's promptness is proven against.
//
// THERE IS ALSO NO MODEL OPTION. Which radio a fake is comes from NewD or NewMP
// and from nothing else (plan D1, and Radio.catID's own comment): an option
// could be forgotten, and a forgotten identity is a fake answering the wrong
// radio's ID.
type Option func(*Radio)

// WithLatency makes every reply the fake sends wait d before being written to
// the port — a per-reply delay, applied once per exchange.
//
// The wait is interruptible: a Close during it abandons the reply and returns
// promptly (Radio.shutdown), so a test may script a multi-second latency without
// a multi-second teardown.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.latency = d
	}
}

// WithFactoryImage REPLACES the fake's entire slot map with img's output. Pass
// it BEFORE any WithSlot, With5xx or WithEMG option in the same constructor
// call, or the image will overwrite them. Without this option, NewD and NewMP
// default to DefaultImage.
//
// It exists for the case internal/wiring's FakeSessionOpts documents for the
// FT-710 and the FTdx10: a test that needs a fake rig with a non-default
// inventory, reached through the EXACT code path a real "--fake" invocation
// uses rather than by hand-building a session that bypasses the constructor.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) {
		r.slots = img()
	}
}

// WithSlot overlays one slot's state onto whatever image is already present.
// Apply it AFTER WithFactoryImage (see there), since the image option replaces
// the whole map.
//
// No validation is applied: the state is stored verbatim, so a test may craft a
// slot whose ANSWER is deliberately malformed — a P7 outside the documented
// {'0','1'} pair, a P11 that is not the fixed '0', a mode nibble the legend
// does not list — and drive a real driver's parse-error path through a real
// fake rather than through a scripted transcript. That is the reason MemState's
// answer-side fields are fields at all (see MemState.Kind and MemState.P11).
func WithSlot(slot string, s MemState) Option {
	return func(r *Radio) {
		r.slots[slot] = s
	}
}
