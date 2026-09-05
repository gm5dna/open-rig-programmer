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

// WithSlot overlays one slot's state onto whatever image is already present.
//
// No validation is applied: the state is stored verbatim, so a test may craft
// a slot whose ANSWER is deliberately malformed — a P5 that is not the fixed
// '0', a P7 outside the tolerated {'0','1'} pair, a mode nibble the legend
// does not list — and drive a real driver's parse-error path through a real
// fake rather than through a scripted transcript. That is the reason
// MemState's answer-side fields are fields at all (see MemState.P5 and
// MemState.Kind).
func WithSlot(slot string, s MemState) Option {
	return func(r *Radio) {
		r.slots[slot] = s
	}
}

// WithEXSetting overlays one EX (MENU) address's raw P4 verbatim — the same
// overlay semantics as WithSlot: it is applied to whatever exSettings already
// holds (EXDefaults(), seeded in New), with no shape or range validation, so
// several WithEXSetting options may be given, including for an address the
// generated inventory does not know about. Such an address becomes answerable
// even though EXDefaults() never produced it, because the option does not
// consult exGroups — which is deliberate: it is how a test reaches a wire
// behaviour the transcription does not describe, WITHOUT editing the projection
// of transcription B that the cross-check depends on. Editing that projection
// to make a test possible would quietly dissolve the cross-check's whole point.
//
// The address is this radio's FOUR digits ("0506"), not a sibling's six.
func WithEXSetting(addr, p4 string) Option {
	return func(r *Radio) {
		r.exSettings[addr] = p4
	}
}

// WithEXUnavailable removes addr from the fake's EX (MENU) address map, applied
// to whatever exSettings already holds (EXDefaults() by default, or a prior
// WithEXSetting in the same Option list), so a subsequent EX read of addr
// answers "?;" — indistinguishable from an address the chart never enumerated
// (ex.go's handleEX, doc.go's register entry AN OUT-OF-INVENTORY EX ADDRESS
// ANSWERS "?;").
//
// It introduces no NEW assumed behaviour: it only removes a map entry, which
// triggers the fake's existing documented "?;". This is the test-only seam for
// forcing a KNOWN, otherwise-valid address to answer as unavailable — what a
// settings reader maps to an unavailable setting — so that such a test need not
// depend on a genuinely out-of-inventory address that no SettingsDescriptor
// would ever offer an ID for in the first place.
func WithEXUnavailable(addr string) Option {
	return func(r *Radio) {
		delete(r.exSettings, addr)
	}
}

// WithMTReadUnsupported makes the fake honour the COMMAND LIST rather than
// MT's detail block: an MT read of any slot answers "?;" while the Set
// direction and MR are untouched.
//
// IT EXISTS BECAUSE THIS MANUAL CONTRADICTS ITSELF and the contradiction is
// the largest unresolved thing about this radio. The Control Command List
// gives MT "Set O, Read X, Ans. X" (ft891_layout.txt:166); MT's own detail
// block, on the same printed page, prints a filled Read chart and a filled
// 41-position Answer chart (1016-1027). Both cannot be true, and
// core/cat/ft891/doc.go records the disagreement without resolving it because
// no FT-891 has been asked. The default fake plays the DETAIL BLOCK; this
// option plays the COMMAND LIST.
//
// What it makes reachable is core/driver/ft891's typed whole-session refusal:
// with the option on, an occupied slot answers "?;" to MT and a record to MR
// in the same session, which is exactly the pair the driver turns into
// ErrMTReadRejectedForOccupiedSlot. Without it that path would have to be
// scripted, and a scripted transcript proves the driver reads its own script.
// TestWithMTReadUnsupported_HonoursTheCommandList pins all three halves.
//
// NOT A FAULT OPTION. It does not model a misbehaving radio; it models the
// other radio this manual describes.
func WithMTReadUnsupported() Option {
	return func(r *Radio) {
		r.mtReadUnsupported = true
	}
}

// WithFactoryImage REPLACES the fake's entire slot map with img's output. Pass
// it BEFORE any WithSlot, With5MHz or WithEMG option in the same New call, or
// the image will overwrite them. Without this option, New defaults to
// DefaultImage.
//
// It exists for the case internal/wiring's per-model FakeSessionOpts variable
// documents: a test that needs a fake rig with a non-default inventory,
// reached through the EXACT code path a real "--fake" invocation uses rather
// than by hand-building a session that bypasses the constructor.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) {
		r.slots = img()
	}
}
