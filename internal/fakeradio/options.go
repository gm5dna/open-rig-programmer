// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import "time"

// Option configures a *Radio at construction time. See New.
type Option func(*Radio)

// WithLatency makes every reply the fake sends wait d before being
// written to the port — a per-reply delay, applied once per exchange (not
// per byte/chunk; see FaultChunkedReplies for chunking, which composes
// with this).
func WithLatency(d time.Duration) Option {
	return func(r *Radio) {
		r.latency = d
	}
}

// WithFactoryImage replaces the fake's entire slot map with img's output.
// It REPLACES rather than merges: pass it before any WithSlot options in
// the New() call, or the factory image will overwrite them. Without this
// option, New defaults to WithFactoryImage(ImageUK).
func WithFactoryImage(img Image) Option {
	return func(r *Radio) {
		r.slots = img()
	}
}

// WithSlot overlays a single slot's state onto whatever image is already
// present, for test setup. Apply it AFTER WithFactoryImage in the Option
// list (see WithFactoryImage) since the image option replaces the whole
// map.
func WithSlot(slot string, s MemState) Option {
	return func(r *Radio) {
		r.slots[slot] = s
	}
}

// WithEXSetting overlays one EX (MENU) address's raw P4 verbatim at
// construction time — same overlay semantics as WithSlot: it is applied
// to whatever exSettings already holds (EXRuntimeDefaults() by default,
// seeded in New), no shape/range validation, so multiple WithEXSetting options
// may be given, including for an address this package's exGroups does
// not know about (making it answerable even though EXDefaults() never
// produced it — the option does not consult exGroups).
func WithEXSetting(addr, p4 string) Option {
	return func(r *Radio) {
		r.exSettings[addr] = p4
	}
}

// WithEXUnavailable removes addr from the fake's EX (MENU) address map —
// applied to whatever exSettings already holds (EXRuntimeDefaults() by
// default, seeded in New, or a prior WithEXSetting in the same
// Option list), so a subsequent EX read of addr answers "?;", exactly as
// an out-of-inventory address already does (doc.go register item 23,
// ex.go's handleEX: an address absent from exSettings is indistinguishable
// from one Table 2 never enumerated). Introduces no NEW assumed
// behaviour — it only removes a map entry, which triggers the fake's
// EXISTING documented "?;" answer for an absent address — so no new
// doc.go register entry is needed; this is a test-only seam for forcing a
// KNOWN, otherwise-valid address to answer as unavailable (e.g.
// core/driver/ft710's TestSession_ReadSetting_Unavailable_Sim), without
// depending on a genuinely out-of-inventory address that no
// SettingsDescriptor would ever offer an ID for in the first place.
func WithEXUnavailable(addr string) Option {
	return func(r *Radio) {
		delete(r.exSettings, addr)
	}
}

// WithFault scripts a deterministic misbehaviour into the fake. Faults
// compose: multiple WithFault options may be given, including several of
// the same kind (e.g. two FaultSpuriousFrame at different exchange
// indices). All fault behaviour is deterministic — no randomness appears
// anywhere in fakeradio.
func WithFault(f Fault) Option {
	return func(r *Radio) {
		f.apply(&r.faults)
	}
}

// Fault is a deterministic misbehaviour script, applied to a *Radio via
// WithFault. See the Fault* constructors.
type Fault interface {
	apply(*faultConfig)
}

// faultConfig is the aggregate of every fault behaviour scripted onto a
// Radio via WithFault. It is populated only while New's options run and
// is never mutated afterwards, so the serve goroutine may read it without
// holding Radio.mu (see fakeradio.go).
//
// "Exchange N" (1-based) below means the Nth complete unit the fake's
// accumulator hands to command processing — the Nth full command frame
// received (ID, AI, MR, MW, MT, MC, an unknown command, or garbage all
// count), OR the Nth accumulator-overflow resync event (see parser.go's
// reassembler). This is a test-harness counting convention, not a radio
// behaviour: it exists purely so Fault indices have an unambiguous
// meaning tied to "the Nth thing the host sent", regardless of whether
// that thing produced a normal reply, no reply, or a "?;".
type faultConfig struct {
	dropRepliesAfterN int // 0 disabled; else no reply from exchange N onward (inclusive)
	garbleReplyN      int // 0 disabled; corrupt exactly exchange N's reply
	spurious          []spuriousFrame
	delayedRejectionN int // 0 disabled; exchange N gets a late "?;" instead of its normal reply
	delayedRejectionD time.Duration
	delayedReplyN     int // 0 disabled; exchange N's ENTIRE reply (whatever it normally is) is delayed
	delayedReplyD     time.Duration
	disconnectAfterN  int // 0 disabled; close the pipe once exchange N has been handled
	chunkedSize       int // 0 disabled (whole reply in one Write); else write in chunks of this many bytes
}

// spuriousFrame is one FaultSpuriousFrame registration.
type spuriousFrame struct {
	frame   []byte
	beforeN int
}

// faultFunc adapts a plain function to the Fault interface.
type faultFunc func(*faultConfig)

func (f faultFunc) apply(fc *faultConfig) { f(fc) }

// FaultDropReplies simulates a timeout: from exchange afterN onward
// (inclusive), the fake silently produces no reply at all, exactly as if
// the radio had stopped responding. Composes with other faults scripted
// for the same or later exchanges (e.g. a garble scripted for an exchange
// at or after afterN never gets a chance to run, since there is no reply
// to garble).
func FaultDropReplies(afterN int) Fault {
	return faultFunc(func(fc *faultConfig) { fc.dropRepliesAfterN = afterN })
}

// FaultGarbleReply corrupts exactly exchange n's reply bytes before
// sending them (deterministically: it flips every bit of the first byte,
// leaving the frame the correct length and still terminated with ';', so
// a corrupt-payload failure is distinguishable from a truncated one).
func FaultGarbleReply(n int) Fault {
	return faultFunc(func(fc *faultConfig) { fc.garbleReplyN = n })
}

// FaultSpuriousFrame injects frame, unprompted, immediately before the
// fake handles exchange beforeN — modelling an unsolicited push arriving
// interleaved with an expected reply. frame is copied; the caller may
// reuse or mutate its argument afterwards. May be scripted more than once
// for different beforeN values (or, for two frames back to back before
// the same exchange, twice with the same beforeN — both fire, in
// registration order).
func FaultSpuriousFrame(frame []byte, beforeN int) Fault {
	cp := append([]byte(nil), frame...)
	return faultFunc(func(fc *faultConfig) {
		fc.spurious = append(fc.spurious, spuriousFrame{frame: cp, beforeN: beforeN})
	})
}

// FaultDelayedRejection makes exchange n reply with "?;" — regardless of
// what its normal reply would have been, including a normally
// fire-and-forget success — only after sleeping d first. This models a
// radio that eventually rejects a command, but too slowly for a host that
// applied a shorter deadline and moved on.
func FaultDelayedRejection(n int, d time.Duration) Fault {
	return faultFunc(func(fc *faultConfig) {
		fc.delayedRejectionN = n
		fc.delayedRejectionD = d
	})
}

// FaultDelayedReply delays exchange n's ENTIRE reply — whatever it would
// normally be (a successful answer, echoed data, or a natural "?;"
// rejection from validation) — by d, before writing it. Unlike
// FaultDelayedRejection, it does NOT override the reply's content, only
// its timing: this models a radio that is genuinely slow to answer,
// rather than one that eventually rejects. Composes with other faults
// scripted for the same exchange (e.g. FaultGarbleReply n: the delay
// happens first, then whatever content transformation the other fault
// applies) and with FaultSpuriousFrame's beforeN injections (which are
// unaffected — they are unconditional, not part of "the reply"). If
// exchange n would normally produce no reply at all (a fire-and-forget
// success), there is nothing to delay.
func FaultDelayedReply(n int, d time.Duration) Fault {
	return faultFunc(func(fc *faultConfig) {
		fc.delayedReplyN = n
		fc.delayedReplyD = d
	})
}

// FaultDisconnect closes the port once exchange afterN has been fully
// handled (its reply, if any, sent first) — modelling the radio going
// silent mid-sequence. Any exchange after afterN never gets a reply,
// since the pipe is gone; a host blocked reading observes io.EOF.
func FaultDisconnect(afterN int) Fault {
	return faultFunc(func(fc *faultConfig) { fc.disconnectAfterN = afterN })
}

// FaultChunkedReplies makes every subsequent reply get written to the
// port in pieces of at most size bytes each (size == 1: byte at a time),
// instead of in one Write call, modelling a host that must reassemble
// replies across arbitrary read boundaries. Applies to ALL replies from
// the moment the Radio is constructed, not to one indexed exchange; it
// composes with WithLatency (the per-reply delay happens once, before the
// first chunk) and with the other faults (a dropped or delayed reply is
// still chunked once it is actually sent).
func FaultChunkedReplies(size int) Fault {
	return faultFunc(func(fc *faultConfig) { fc.chunkedSize = size })
}
