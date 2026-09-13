// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx1200

import "time"

// Option configures a *Radio at construction time. See New.
type Option func(*Radio)

// defaultCATID and fft1NotFittedCATID are the two ID answers this ONE radio
// gives, "P1 0582: FTDX1200 (optional FFT-1 is installed) / 0583: FTDX1200
// (optional FFT-1 is not installed)" (matrix §2.2) — the only byte-level
// difference the manual states, an internal option-split rather than two
// model rows.
const (
	defaultCATID       = "0582"
	fft1NotFittedCATID = "0583"
)

// WithFFT1NotFitted makes this Radio answer ID as "0583" instead of the
// default "0582" — the option-split's other value, matrix §2.2. Nothing
// else about this radio's record, legends or ranges differs between the two
// IDs.
func WithFFT1NotFitted() Option {
	return func(r *Radio) { r.catID = fft1NotFittedCATID }
}

// WithLatency makes every reply the fake sends wait d before being written
// to the port. The wait is interruptible — a Close mid-wait abandons the
// reply (Radio.pipe's own promptness guarantee).
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.pipe.Latency = d }
}

// WithSlot overlays one slot's state onto whatever image is already
// present. No validation is applied: the state is stored verbatim, so a
// test may craft an answer this fake is ASSUMED never to give (an
// out-of-legend Mode or CTCSS byte) and drive a real driver's parse-error
// path through a real fake rather than a scripted transcript.
func WithSlot(slot string, s MemState) Option {
	return func(r *Radio) { r.slots[slot] = s }
}

// WithFactoryImage REPLACES the fake's entire slot map with img's output.
// Pass it before any WithSlot option in the same New call, or the image
// will overwrite them. Without this option, New defaults to DefaultImage.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) { r.slots = img() }
}
