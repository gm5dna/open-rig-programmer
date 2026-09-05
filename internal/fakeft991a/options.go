// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

import "time"

// Option configures a *Radio at construction time. See New.
//
// THE SET IS THE SMALLEST OF ANY FAKE IN THIS REPOSITORY, and every absence is
// a decision rather than an omission:
//
//   - NO FAULT INJECTION. doc.go's "What this fake deliberately does NOT
//     model" lists the seven faults internal/fakeradio carries and states why
//     none is copied — in short, they exercise core/transport.Engine, which is
//     one model-independent implementation already covered by fakeradio's
//     fault suite, and no wiring, CLI or GUI path uses faults against a fake
//     rig. WithLatency stays because it is not a fault: it is the knob Close's
//     promptness is proven against.
//   - NO WithMTReadUnsupported(). internal/fakeft891 has one because that
//     manual contradicts itself about MT's very availability. THIS ONE DOES
//     NOT: the availability row gives MT "O O O X" (ft991a_layout.txt:181) and
//     its own detail block prints a Read chart and a full Answer chart
//     (998-1033), and the two agree. There is no second radio to play, and
//     this milestone's plan decision P14 says so in terms.
//   - NO EX OPTIONS. EX is not modelled yet at all (doc.go).
//   - NO BANK OPTIONS. internal/fakeft891 has With5MHz and WithEMG because
//     that radio's legends print those banks; "5xx", "5 MHz" and "EMG" appear
//     in no slot legend of this manual, so there is nothing to populate.
type Option func(*Radio)

// WithLatency makes every reply the fake sends wait d before being written to
// the port — a per-reply delay, applied once per exchange.
//
// The wait is interruptible: a Close during it abandons the reply and returns
// promptly (Radio.shutdown), so a test may script a multi-second latency
// without a multi-second teardown — TestClose_IsPromptDespiteAPendingLatency.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.latency = d
	}
}

// WithSlot overlays one slot's state onto whatever image is already present.
//
// No validation is applied: the state is stored verbatim, so a test may craft
// a slot whose ANSWER is deliberately malformed — a P11 that is not the fixed
// '0', a P7 outside the printed {'0','1'} pair, a mode nibble the legend does
// not list — and drive a real driver's parse-error path through a real fake
// rather than through a scripted transcript. That is the reason MemState's
// answer-side fields are fields at all (see MemState.P11 and MemState.Kind).
func WithSlot(slot string, s MemState) Option {
	return func(r *Radio) {
		r.slots[slot] = s
	}
}

// WithFactoryImage REPLACES the fake's entire slot map with img's output. Pass
// it BEFORE any WithSlot or WithDCSChannels option in the same New call, or
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

// dcsChannelSlots are the two memory channels WithDCSChannels populates, one
// per DCS state. Both are absent from DefaultImage, so the option adds rather
// than shadows.
var dcsChannelSlots = []string{"003", "004"}

// WithDCSChannels populates one memory channel at P8 '3' (DCS ENC/DEC) and one
// at P8 '4' (DCS ENC), at invented placeholder frequencies in FM — the two
// states of the five-value P8 legend that the DEFAULT IMAGE DELIBERATELY LACKS.
//
// THE SPLIT IS THIS MILESTONE'S PLAN, DECISION P14, and the reason is a fleet
// one rather than a taste one: this radio's P8 is the first Yaesu memory
// record in this project with a DCS state, and a default image carrying one
// would push a value through every fleet-wide pin that predates the five-state
// vocabulary. So the default image round-trips as any sibling's would, and a
// test that wants the new axis asks for it here.
//
// The frequencies and the mode are placeholders like every other value in this
// package's fixtures (doc.go's register entry THE DEFAULT IMAGE'S CONTENT IS
// INVENTED): no FT-991A's memory contents have been read, and DCS is a value
// of P8 rather than a property of a band.
//
// Overlay semantics, like WithSlot: it adds to whatever image is already
// present, so it must be given AFTER any WithFactoryImage in the same New
// call.
func WithDCSChannels() Option {
	return func(r *Radio) {
		for i, state := range []byte{'3', '4'} {
			s := defaultState(uint64(145_500_000+i*25_000), modeFM)
			s.CTCSS = state
			r.slots[dcsChannelSlots[i]] = s
		}
	}
}
