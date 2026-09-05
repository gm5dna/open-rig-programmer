// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

import (
	"fmt"
	"time"
)

// Option configures a *Radio at construction time. See New.
//
// THE SET IS SMALL, AND IT IS NOT internal/fakeradio's FAULT SUITE. The
// general transport faults — dropped replies, garbled bytes, spurious frames,
// chunked writes — exercise core/transport.Engine, one model-independent
// implementation already covered against fakeradio, and nothing is learnt by
// running them past a second dialect. What IS here is the set of behaviours
// THIS BOOK describes and the Kenwood driver has to answer for: the two
// serial-line error tokens (590:110-113), the transient "?;" suppression the
// error table itself prints (590:106-108), and the firmware string the
// TS-590S's write refusals turn on (590:1030-1037). WithLatency stays because
// it is not a fault: it is the knob Close's promptness is proven against.
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

// WithFirmwareVersion sets the four characters this radio answers to "FV;"
// (590:1030-1037), replacing the book's worked example "1.00" (590:1035).
//
// IT EXISTS TO MAKE TWO DRIVER REFUSALS REACHABLE THROUGH A REAL FAKE. On the
// TS-590S row, channel writes are refused when the FV answer is 2.00 or later
// (A14 — byte 28 may be live there, 590:1478) or when it does not parse as
// A13's assumed "M.NN" form. Without this option both rungs would have to be
// pinned against a scripted transcript, and a scripted transcript proves the
// driver reads its own script.
//
// The string is answered VERBATIM and no grammar is applied to it: deciding
// what "2.00" or "1.xx" means is the driver's job, and a fake that validated
// the field would be asserting A13 as a fact about the radio. The WIDTH is
// enforced, because the chart counts four P1 bytes and a field of any other
// width could not be sent by any radio; a bad fixture panics, which is
// defaultRecord's reasoning — every call site passes a constant.
func WithFirmwareVersion(s string) Option {
	if len(s) != firmwareFieldLen {
		panic(fmt.Sprintf("fakets590: firmware version %q is %d bytes; FV's P1 field is %d (590:1037)", s, len(s), firmwareFieldLen))
	}
	return func(r *Radio) {
		r.firmware = s
	}
}

// WithTransientNAKSuppressed makes the fake DROP every "?;" it would
// otherwise send, answering nothing at all instead.
//
// IT IS THE BOOK'S OWN NOTE, PLAYED. The error table prints, under the "?;"
// row itself: "Note: Occasionally, this message may not appear due to
// microprocessor transients in the transceiver." (590:106-108). So a Kenwood
// host cannot treat silence as "the radio did not refuse" — a timeout and a
// rejection are the same event seen twice — and the driver's typed read
// failure has to name both causes.
//
// NOT A FAULT OPTION. It does not model a misbehaving radio; it models the
// radio this book describes, on the branch the book itself flags. Accepted
// answers and fire-and-forget successes are untouched.
func WithTransientNAKSuppressed() Option {
	return func(r *Radio) {
		r.transientNAKSuppressed = true
	}
}

// WithStreamError scripts one of the two SERIAL-LINE error tokens
// (590:110-113) in place of exchange n's reply, where n counts complete
// frames the fake has handled from 1.
//
// The tokens are not command outcomes: "E;" reports "a communication error
// ... such as an overrun or framing error during a serial data transmission"
// (590:110-112) and "O;" a receive buffer overrun (590:113). They therefore
// REPLACE whatever the exchange would have produced, including a
// fire-and-forget silence, and they are not suppressed by
// WithTransientNAKSuppressed, whose sentence is about "?;" alone.
//
// The kind is required: StreamErrorUnset panics rather than defaulting to one
// of the two tokens, which would put a test on the wrong sentence of the
// book. n below 1 panics for the same reason — there is no exchange 0.
func WithStreamError(kind StreamError, n int) Option {
	if kind != StreamErrorE && kind != StreamErrorO {
		panic(fmt.Sprintf("fakets590: WithStreamError(%v) — the token is REQUIRED; the two are printed with different causes (590:110-113)", kind))
	}
	if n < 1 {
		panic(fmt.Sprintf("fakets590: WithStreamError exchange %d — exchanges are counted from 1", n))
	}
	return func(r *Radio) {
		r.streamErrors[n] = kind
	}
}
