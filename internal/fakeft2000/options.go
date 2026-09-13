// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft2000

import "time"

// Option configures a *Radio at construction time. See New.
type Option func(*Radio)

// modelCATID maps this package's two rows to their ID answer, "P1 0251:
// FT-2000 / 0252: FT-2000D" (layout:725-726) — the one byte-level difference
// either book states between them.
var modelCATID = map[string]string{
	"FT-2000":  "0251",
	"FT-2000D": "0252",
}

// WithModelName selects which of the two registered rows this Radio answers
// ID as — a plain Option, not a required constructor argument, the mechanism
// internal/fakeic7851's WithModelName uses and this milestone's DELIBERATE
// DEVIATION from internal/fakets590's required Row argument (doc.go). name
// must be "FT-2000" or "FT-2000D"; anything else panics, since there is no
// third CATID either book prints for this fake to fall back to.
func WithModelName(name string) Option {
	catID, ok := modelCATID[name]
	if !ok {
		panic("fakeft2000: unknown model name " + name + ", want \"FT-2000\" or \"FT-2000D\"")
	}
	return func(r *Radio) {
		r.model = name
		r.catID = catID
	}
}

// WithLatency makes every reply the fake sends wait d before being written to
// the port. The wait is interruptible — a Close mid-wait abandons the reply
// (Radio.pipe's own promptness guarantee).
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.pipe.Latency = d }
}

// WithSlot overlays one slot's state onto whatever image is already present.
// No validation is applied: the state is stored verbatim, so a test may craft
// an answer this fake is ASSUMED never to give (an out-of-legend Mode or
// CTCSS byte, a Tone outside two digits) and drive a real driver's
// parse-error path through a real fake rather than a scripted transcript.
func WithSlot(slot string, s MemState) Option {
	return func(r *Radio) { r.slots[slot] = s }
}

// WithFactoryImage REPLACES the fake's entire slot map with img's output.
// Pass it before any WithSlot option in the same New call, or the image will
// overwrite them. Without this option, New defaults to DefaultImage.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) { r.slots = img() }
}
