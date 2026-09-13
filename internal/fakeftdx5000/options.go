// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx5000

import "time"

// WithLatency makes every reply the fake sends wait d before being written
// to the port — a per-reply delay, applied once per exchange. The wait is
// interruptible: a Close during it abandons the reply and returns promptly.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.pipe.Latency = d
	}
}

// WithSlot overlays one slot's state before the radio starts serving —
// the same "no validation, stored verbatim" overlay internal/fakedx101's
// WithSlot uses, so a test may craft a slot with a mode nibble the legend
// does not list, or override doc.go register entry 1's zero-record
// default with different content, and drive a real driver's behaviour
// against it through a real fake.
//
// slot must be the 3-digit ASCII form MR/MW themselves use ("001".."117");
// this package does no zero-padding or range check on the key.
func WithSlot(slot string, s MemState) Option {
	return func(r *Radio) {
		r.slots[slot] = s
	}
}
