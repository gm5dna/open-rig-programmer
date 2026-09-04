// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

import "time"

// Option configures a *Radio at construction time. See New.
//
// THE SET IS SMALL, AND SMALLER THAN internal/fakeradio's ON PURPOSE: there
// is no fault injection here at all. doc.go's "What this fake deliberately
// does NOT model" section lists the seven faults fakeradio carries and states
// why none is copied — in short, they exercise core/transport.Engine, which
// is one model-independent implementation already covered by fakeradio's
// fault suite, and no wiring, CLI or GUI path uses faults against a fake rig.
// WithLatency stays because it is not a fault: it is the knob Close's
// promptness is proven against.
type Option func(*Radio)

// WithLatency makes every reply the fake sends wait d before being written to
// the port — a per-reply delay, applied once per exchange.
//
// The wait is interruptible: a Close during it abandons the reply and returns
// promptly (Radio.shutdown), so a test may script a multi-second latency
// without a multi-second teardown —
// TestClose_IsPromptDespiteAPendingLatency.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.latency = d
	}
}
