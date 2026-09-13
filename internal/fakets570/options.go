// SPDX-License-Identifier: GPL-3.0-or-later

package fakets570

import (
	"fmt"
	"time"
)

// Option configures a *Radio at construction time. See New.
type Option func(*Radio)

// WithModelName selects the row a *Radio plays: "TS-570D" (New's own
// default), "TS-570S" or "TS-570DG". Any other string panics — a
// construction-time programming error, not a silently-wrong row — per
// internal/fakeic7851's option mechanism, doc.go's "Three rows, one Option".
func WithModelName(name string) Option {
	catID, ok := map[string]string{
		"TS-570D":  "017",
		"TS-570S":  "018",
		"TS-570DG": "000", // ASSUMED placeholder — doc.go register entry 6
	}[name]
	if !ok {
		panic(fmt.Sprintf("fakets570: WithModelName(%q) — this package plays TS-570D, TS-570S or TS-570DG, and no other row", name))
	}
	return func(r *Radio) {
		r.row = name
		r.catID = catID
	}
}

// WithCATID overrides the row's CATID answer directly. It exists for
// TS-570DG alone (doc.go register entry 6): no document assigns that row a
// CATID, so "000" is a placeholder rather than evidence, and this option is
// the lift for a DG-specific document or a corroborating ID capture
// surfacing without waiting for this package to change.
func WithCATID(id string) Option {
	if len(id) != 3 || !allDigits(id) {
		panic(fmt.Sprintf("fakets570: WithCATID(%q) — the ID answer's P1 is exactly three ASCII digits (matrix §2, PDF p.83 printed 77)", id))
	}
	return func(r *Radio) { r.catID = id }
}

// WithLatency makes every reply the fake sends wait d before being written to
// the port — a per-reply delay, applied once per exchange. Not a fault: the
// knob Close's promptness is proven against, same shape as the sibling
// Kenwood fakes' own WithLatency.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.pipe.Latency = d }
}

// WithChannel overlays one half of one channel, applied to whatever the
// records map already holds (New starts empty, so every channel answers the
// zero record until written or staged). Equivalent to calling SetChannel
// before any host has connected.
func WithChannel(channel int, half Half, s MemState) Option {
	return func(r *Radio) { r.records[recordKey{channel: channel, half: half}] = s }
}

// WithEmptyChannel removes both halves of channel from the records map, so a
// subsequent MR of either half answers the zero record. It introduces no new
// assumed behaviour of its own — it only removes map entries, triggering the
// fake's existing zero-record answer (doc.go register entry 2). Useful after
// a WithChannel in the same New call, to force one channel back to empty.
func WithEmptyChannel(channel int) Option {
	return func(r *Radio) {
		delete(r.records, recordKey{channel: channel, half: HalfRXOrStart})
		delete(r.records, recordKey{channel: channel, half: HalfTXOrEnd})
	}
}
