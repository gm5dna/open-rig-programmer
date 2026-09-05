// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

import (
	"fmt"
	"strings"
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
// serial-line error tokens (480:140-144), the transient "?;" suppression the
// error table itself prints (480:136-138), and the TY answer a probe reads
// (480:1621-1634). WithLatency stays because it is not a fault: it is the
// knob Close's promptness is proven against.
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

// WithTYAnswer sets the two reserved bytes and the variant digit this radio
// answers to "TY;" (480:1621-1634), replacing the shipped "00" and '0'.
//
// IT EXISTS TO MAKE TWO DRIVER PATHS REACHABLE THROUGH A REAL FAKE. The
// first is the session refusal of a FIFTH variant: this document prints
// exactly four, '0'..'3' (480:1626-1629), and decision 4 refuses anything
// else rather than reporting it as unknown or defaulting it to one of the
// four — a refusal that cannot be exercised unless something can answer a
// '4'. The second is P1's OPAQUE bytes, which are the one field in this
// family admitted above 0x7E, and which a caller rendering a probe note must
// %q-quote. Without this option both would have to be pinned against a
// scripted transcript, and a scripted transcript proves the driver reads its
// own script.
//
// THE BYTES ARE ANSWERED VERBATIM AND NO GRAMMAR IS APPLIED. Deciding what a
// variant digit means is the driver's job, and a fake that validated the
// field would be asserting a grammar this document does not print — for P1
// it prints one word, "Reserved" (480:1623).
//
// TWO FIXTURE SHAPES PANIC, and neither is a value judgement about the
// bytes. P1 is TWO bytes on the wire (480:1634), so a field of any other
// width could not be sent by any radio; and a ';' anywhere in the answer is a
// SECOND FRAME to the host's own reassembler rather than a byte of this one,
// which is also this document's own general rule for a parameter
// (480:108-111, 480:127-129). Everything else is admitted, control codes
// included, so that a driver's refusal of those is reachable too. A bad
// fixture panics, which is New's reasoning one layer up: every call site
// passes a compile-time-known constant.
func WithTYAnswer(reserved string, variant byte) Option {
	if len(reserved) != tyReservedLen {
		panic(fmt.Sprintf("fakets480: TY reserved field %q is %d bytes; P1 is %d (480:1634)", reserved, len(reserved), tyReservedLen))
	}
	if strings.ContainsRune(reserved, ';') || variant == ';' {
		panic(fmt.Sprintf("fakets480: TY answer %q/%q carries a ';' — the terminator ends a frame (480:113-118), so such an answer is two frames and not one", reserved, variant))
	}
	return func(r *Radio) {
		r.tyReserved = reserved
		r.tyVariant = variant
	}
}

// WithTransientNAKSuppressed makes the fake DROP every "?;" it would
// otherwise send, answering nothing at all instead.
//
// IT IS THE BOOK'S OWN NOTE, PLAYED. The error table prints, under the "?;"
// row itself: "Note: Occasionally this message may not appear due to
// microprocessor transients in the transceiver." (480:136-138). So a Kenwood
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
// (480:140-144) in place of exchange n's reply, where n counts EVENTS the
// fake has handled from 1 — an accumulator overflow counts too, since
// handleEvent increments before it knows whether the event is a complete
// frame.
//
// The tokens are not command outcomes: "E;" reports "A communication error
// occurred such as an overrun or framing error during a serial data
// transmission" (480:140-142) and "O;" that "Receive data was sent but
// processing was not completed" (480:143-144). They therefore REPLACE
// whatever the exchange would have produced, including a fire-and-forget
// silence, and they are not suppressed by WithTransientNAKSuppressed, whose
// sentence is about "?;" alone.
//
// The kind is required: StreamErrorUnset panics rather than defaulting to one
// of the two tokens, which would put a test on the wrong sentence of the
// book. n below 1 panics for the same reason — there is no exchange 0.
func WithStreamError(kind StreamError, n int) Option {
	if kind != StreamErrorE && kind != StreamErrorO {
		panic(fmt.Sprintf("fakets480: WithStreamError(%v) — the token is REQUIRED; the two are printed with different causes (480:140-144)", kind))
	}
	if n < 1 {
		panic(fmt.Sprintf("fakets480: WithStreamError exchange %d — exchanges are counted from 1", n))
	}
	return func(r *Radio) {
		r.streamErrors[n] = kind
	}
}
