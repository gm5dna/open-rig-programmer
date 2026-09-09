// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

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
// THIS BOOK describes and this row's driver has to answer for: the two
// serial-line error tokens (890:118-123), the transient "?;" suppression the
// error table itself prints (890:114-116), and the seams a test needs to stage
// a channel or a menu the image does not happen to carry. WithLatency stays
// because it is not a fault: it is the knob Close's promptness is proven
// against.
type Option func(*Radio)

// WithLatency makes every reply the fake sends wait d before being written to
// the port — a per-reply delay, applied once per exchange.
//
// The wait is interruptible: a Close during it abandons the reply and returns
// promptly, so a test may script a multi-second latency without a
// multi-second teardown — TestClose_IsPromptDespiteAPendingLatency.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.pipe.Latency = d
	}
}

// WithFactoryImage REPLACES the fake's entire record map with img's output.
// Pass it BEFORE any WithChannel or WithEmptyChannel option in the same New
// call, or the image will overwrite them. Without this option, New defaults to
// DefaultImage.
//
// It exists for the case internal/wiring's per-model FakeSessionOpts variable
// documents: a test that needs a fake rig with a non-default inventory,
// reached through the EXACT code path a real "--fake" invocation uses rather
// than by hand-building a session that bypasses the constructor.
func WithFactoryImage(img Image) Option {
	return func(r *Radio) {
		r.records = img()
	}
}

// WithChannel overlays ONE channel's record.
//
// No validation is applied: the record is stored verbatim, so a test may craft
// a channel whose ANSWER is deliberately malformed — a mode nibble the OM
// legend does not print, a tone index above the printed chart, a name byte
// outside A2's charset — and drive a real driver's parse-error path through a
// real fake rather than through a scripted transcript. That is why MemState's
// fields are raw wire bytes at all.
//
// Overlay semantics: it is applied to whatever record map is already present.
func WithChannel(channel int, s MemState) Option {
	return func(r *Radio) {
		r.records[channel] = s
	}
}

// WithEmptyChannel removes channel from the record map, so a subsequent MA0
// read of it answers the blank frame — "When reading a blank channel,
// parameters P2 to P12 becomes blank." (890:3215-3216).
//
// IT INTRODUCES NO NEW ASSUMED BEHAVIOUR: it only removes a map entry, which
// triggers the fake's existing documented blank-channel answer. This is the
// test-only seam for forcing a channel the default image populates to read
// back blank, so that a driver's blank-channel handling can be pinned against
// a channel a test names rather than against whichever channel the image
// happens not to fill.
//
// Overlay semantics: it is applied to whatever record map is already present,
// so it must be given AFTER any WithFactoryImage in the same New call.
func WithEmptyChannel(channel int) Option {
	return func(r *Radio) {
		delete(r.records, channel)
	}
}

// WithMemoryReadUnsupported makes an MA0 read of ANY channel answer "?;" while
// an MA0 Set is untouched.
//
// IT MAKES THE READ LADDER'S RULE REACHABLE END TO END. A "?;" on this radio
// is a definitive rejection: it is never retried, and it is never read as "the
// channel is blank". With this option a session can read a rejection for every
// channel, which is what drives core/driver/ts890's typed whole-read failure
// through a real fake instead of a scripted transcript.
//
// NOT A CLAIM THAT ANY TS-890S REFUSES MA0. It plays the SECOND cause the
// error table itself prints — "Command was not executed due to the current
// status of the transceiver (even though the command syntax was correct)"
// (890:108-112) — which is a state, not a defect.
func WithMemoryReadUnsupported() Option {
	return func(r *Radio) {
		r.memoryReadUnsupported = true
	}
}

// WithTransientNAKSuppressed makes the fake DROP every "?;" it would otherwise
// send, answering nothing at all instead.
//
// IT IS THE BOOK'S OWN NOTE, PLAYED. The error table prints, under the "?;"
// row itself: "Note: Occasionally, this message may not appear due to
// microprocessor transients in the transceiver." (890:114-116). So a Kenwood
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
// (890:118-123) in place of exchange n's reply, where n counts EVENTS the fake
// has handled from 1 — an accumulator overflow counts too, since handleEvent
// increments before it knows whether the event is a complete frame.
//
// The tokens are not command outcomes: "E;" reports "a communication error
// ... such as an overrun or framing error during a serial data transmission"
// (890:118-120) and "O;" a receive buffer overrun (890:121-123). core/kw's
// framing treats both as FATAL to the session and types them with this book's
// own cause sentences, so this option is the only way to put one on a wire
// from a real TS-890S fake. They REPLACE whatever the exchange would have
// produced, including a fire-and-forget silence, and they are not suppressed
// by WithTransientNAKSuppressed, whose sentence is about "?;" alone.
//
// The kind is required: StreamErrorUnset panics rather than defaulting to one
// of the two tokens, which would put a test on the wrong sentence of the book.
// n below 1 panics for the same reason — there is no exchange 0.
func WithStreamError(kind StreamError, n int) Option {
	if kind != StreamErrorE && kind != StreamErrorO {
		panic(fmt.Sprintf("fakets890: WithStreamError(%v) — the token is REQUIRED; the two are printed with different causes (890:118-123)", kind))
	}
	if n < 1 {
		panic(fmt.Sprintf("fakets890: WithStreamError exchange %d — exchanges are counted from 1", n))
	}
	return func(r *Radio) {
		r.streamErrors[n] = kind
	}
}

// WithEXSetting overlays one EX (MENU) address's raw P5 verbatim — the same
// overlay semantics as WithChannel: it is applied to whatever exSettings
// already holds (EXDefaults(), seeded in New), so several WithEXSetting
// options may be given, and a later one wins.
//
// IT IS DELIBERATELY LOOSE ABOUT MEMBERSHIP AND WIDTH. The option does not
// consult the projected widths table, so an address this chart does not print
// becomes answerable, and a P5 shorter or wider than the printed width can be
// scripted — WITHOUT editing the projection of transcription B that
// core/transport's cross-check depends on. Both halves earn their keep on this
// row: A19 claims a MAXIMUM and nothing else, so the codec's parser admits a
// short answer and must be exercised with one, and the same parser REFUSES an
// over-wide answer, which can only be driven from a fake willing to send one.
//
// IT IS STRICT ABOUT THE ADDRESS'S SHAPE, and that is not the same thing. This
// row's wire address is FIVE characters, P1 + P2P2 + P3P3 (890:1900,
// 890:1907, 890:1910), so an entry keyed "087" could never be reached by
// handleEX and would read as a working overlay that silently did nothing.
// Every call site passes a literal, so a malformed address panics.
//
// The bytes it stores are INVENTED, like the defaults they replace: doc.go's
// register entry THE EX MENU VALUES ARE INVENTED covers both.
func WithEXSetting(addr, p5 string) Option {
	mustBeEXAddr("WithEXSetting", addr)
	return func(r *Radio) {
		r.exSettings[addr] = p5
	}
}

// WithEXUnavailable removes addr from the fake's EX (MENU) address map,
// applied to whatever exSettings already holds, so a subsequent EX read of
// addr answers "?;" — indistinguishable from a menu number this chart never
// printed (ex.go's handleEX, doc.go's register entry AN OUT-OF-INVENTORY EX
// ADDRESS ANSWERS "?;").
//
// IT INTRODUCES NO NEW ASSUMED BEHAVIOUR: it only removes a map entry, which
// triggers the fake's existing "?;". This is the test-only seam for forcing a
// KNOWN, otherwise-valid menu to answer as unavailable — what a settings
// reader maps to an unavailable setting — so that a partial snapshot can be
// built from menus a test names rather than from whichever addresses the chart
// happens not to have.
//
// The address's SHAPE is checked for WithEXSetting's reason: a delete keyed
// "42" removes nothing and reads as a working option.
func WithEXUnavailable(addr string) Option {
	mustBeEXAddr("WithEXUnavailable", addr)
	return func(r *Radio) {
		delete(r.exSettings, addr)
	}
}

// mustBeEXAddr panics unless addr is exactly five ASCII digits whose first is
// one of the two printed menu types — the whole of this row's EX wire address
// (890:1897-1911).
func mustBeEXAddr(option, addr string) {
	if len(addr) != exAddrLen || !allDigits(addr) || (addr[0] != '0' && addr[0] != '1') {
		panic(fmt.Sprintf("fakets890: %s(%q) — this row's EX address is five ASCII digits, a menu type of 0 or 1 followed by a two-digit category and a two-digit item (890:1897-1911), and any other key is unreachable from handleEX", option, addr))
	}
}
