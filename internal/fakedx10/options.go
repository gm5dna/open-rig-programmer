// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx10

import "time"

// Option configures a *Radio at construction time. See New.
//
// THE SET IS SMALL, AND SMALLER THAN fakeradio's ON PURPOSE: there is no fault
// injection here at all. doc.go's "What this fake deliberately does NOT model"
// section lists the seven faults fakeradio carries and states why none is
// copied — in short, they exercise core/transport.Engine, which is one
// model-independent implementation already covered by fakeradio's fault suite,
// and no wiring, CLI or GUI path uses faults against a fake rig. WithLatency
// stays because it is not a fault: it is the knob Close's promptness is proven
// against.
type Option func(*Radio)

// WithLatency makes every reply the fake sends wait d before being written to
// the port — a per-reply delay, applied once per exchange.
//
// The wait is interruptible: a Close during it abandons the reply and returns
// promptly (Radio.shutdown), so a test may script a multi-second latency
// without a multi-second teardown.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.latency = d
	}
}

// WithFactoryImage REPLACES the fake's entire slot map with img's output. Pass
// it BEFORE any WithSlot, With5xx or WithEMG option in the same New call, or
// the image will overwrite them. Without this option, New defaults to
// DefaultImage.
//
// It exists for the case internal/wiring's FakeSessionOpts documents for the
// FT-710: a test that needs a fake rig with a non-default inventory, reached
// through the EXACT code path a real "--fake" invocation uses rather than by
// hand-building a session that bypasses the constructor.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) {
		r.slots = img()
	}
}

// WithSlot overlays one slot's state onto whatever image is already present.
// Apply it AFTER WithFactoryImage (see there), since the image option replaces
// the whole map.
//
// No validation is applied: the state is stored verbatim, so a test may craft
// a slot whose ANSWER is deliberately malformed — a P7 outside the documented
// {'0','1'} pair, a P11 that is not the fixed '0', a mode nibble the legend
// does not list — and drive a real driver's parse-error path through a real
// fake rather than through a scripted transcript. That is the reason
// MemState's answer-side fields are fields at all (see MemState.Kind and
// MemState.P11).
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
// behaviour the transcription does not describe, without editing the projection
// of transcription B that the cross-check depends on.
func WithEXSetting(addr, p4 string) Option {
	return func(r *Radio) {
		r.exSettings[addr] = p4
	}
}

// WithEXUnavailable removes addr from the fake's EX (MENU) address map, applied
// to whatever exSettings already holds (EXDefaults() by default, or a prior
// WithEXSetting in the same Option list), so a subsequent EX read of addr
// answers "?;" — indistinguishable from an address the chart never enumerated
// (ex.go's handleEX, doc.go register entry 17).
//
// It introduces no NEW assumed behaviour: it only removes a map entry, which
// triggers the fake's existing documented "?;". This is the test-only seam for
// forcing a KNOWN, otherwise-valid address to answer as unavailable — what
// core/driver/ftdx10's settings reader maps to driver.SettingUnavailable — so
// that such a test need not depend on a genuinely out-of-inventory address that
// no SettingsDescriptor would ever offer an ID for in the first place.
func WithEXUnavailable(addr string) Option {
	return func(r *Radio) {
		delete(r.exSettings, addr)
	}
}
