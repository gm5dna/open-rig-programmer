// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftx1

import "time"

// Option configures a *Radio at construction time. See New.
type Option func(*Radio)

// catID is the one and only ID answer this package gives, "0840 (Fixed)"
// (spec.md §1) — no body/model option exists: no CAT mechanism tells
// "field" from "optima" apart (doc.go, BODY IDENTITY IS NEVER SURFACED).
const catID = "0840"

// WithLatency makes every reply the fake sends wait d before being written
// to the port. The wait is interruptible — a Close mid-wait abandons the
// reply (Radio.pipe's own promptness guarantee).
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.pipe.Latency = d }
}

// WithSlot overlays one address's MR/MW memory-block state onto whatever
// image is already present. No validation is applied: the state is stored
// verbatim, so a test may craft an answer this fake is ASSUMED never to
// give and drive a real driver's parse-error or read path through a real
// fake rather than a scripted transcript.
func WithSlot(addr string, s MemState) Option {
	return func(r *Radio) { r.slots[addr] = s }
}

// WithFactoryImage REPLACES the fake's entire slot AND tag maps with img's
// output. Pass it before any WithSlot option in the same New
// call, or the image will overwrite them. Without this option, New
// defaults to DefaultImage.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) { r.slots, r.tags = img() }
}
