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
		"TS-570D": "017",
		"TS-570S": "018",
		// ASSUMED, inherited from core/driver/ts570's own placeholder
		// (modelDG.catID == modelD.catID, ts570.go:52; reviews/driver-ts570.md
		// deviation 7), not the manual — doc.go register entry 6.
		"TS-570DG": "017",
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
// CATID, so "017" (inherited from the driver's own same placeholder) is not
// evidence, and this option is the lift for a DG-specific document or a
// corroborating ID capture surfacing without waiting for this package to
// change.
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

// WithStreamError scripts one of the two serial-line error tokens ("E;" or
// "O;", printed folio 70 — see StreamError) in place of exchange n's reply,
// where n counts events the fake has handled from 1 (an accumulator overflow
// counts too). It replaces whatever the exchange would otherwise have
// produced, including a fire-and-forget silent success.
//
// This is now a LIVE wire path (doc.go register entry 7, updated per the
// lift-K follow-up, commit e7515d0): core/kw's Book570 now carries a cited
// stream-error entry for this same manual span, so a real driver session may
// reach it, and this fake must be able to script it for the cross-check to
// drive that path against something real.
//
// The kind is required: StreamErrorUnset panics rather than defaulting to one
// of the two tokens, which would put a test on the wrong sentence of the
// manual. n below 1 panics for the same reason — there is no exchange 0.
func WithStreamError(kind StreamError, n int) Option {
	if kind != StreamErrorE && kind != StreamErrorO {
		panic(fmt.Sprintf("fakets570: WithStreamError(%v) — the token is REQUIRED; the two are printed with different causes (printed folio 70)", kind))
	}
	if n < 1 {
		panic(fmt.Sprintf("fakets570: WithStreamError exchange %d — exchanges are counted from 1", n))
	}
	return func(r *Radio) { r.streamErrors[n] = kind }
}
