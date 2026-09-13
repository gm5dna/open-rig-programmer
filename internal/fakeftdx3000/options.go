// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx3000

import "time"

// Option configures a *Radio at construction time. See New.
type Option func(*Radio)

// catID is the one and only ID answer this package's single row gives,
// "P1 0462: FTDX3000" (layout:759) — no WithModelName option exists,
// unlike internal/fakeft2000's two-row shape, because the manual states
// exactly one row (doc.go).
const catID = "0462"

// WithLatency makes every reply the fake sends wait d before being written
// to the port. The wait is interruptible — a Close mid-wait abandons the
// reply (Radio.pipe's own promptness guarantee).
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.pipe.Latency = d }
}

// WithSlot overlays one slot's state onto whatever image is already
// present. No validation is applied: the state is stored verbatim, so a
// test may craft an answer this fake is ASSUMED never to give (an
// out-of-legend Mode or CTCSS byte, a nonzero Tone — the only way a stored
// slot's P9 becomes live, doc.go register entry 3) and drive a real
// driver's parse-error or read path through a real fake rather than a
// scripted transcript.
func WithSlot(slot string, s MemState) Option {
	return func(r *Radio) { r.slots[slot] = s }
}

// WithFactoryImage REPLACES the fake's entire slot map with img's output.
// Pass it before any WithSlot option in the same New call, or the image
// will overwrite them. Without this option, New defaults to DefaultImage.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) { r.slots = img() }
}
