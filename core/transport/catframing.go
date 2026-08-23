// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import "github.com/gm5dna/open-rig-programmer/core/cat"

// catFraming is the Framing adapter for Yaesu CAT: the six protocol touch
// points D2 names, each delegating to core/cat exactly as Engine's own
// code did inline before the seam existed. It lives HERE, not in core/cat,
// because core/transport already imports core/cat for precisely these
// things (see doc.go) and the reverse import would cycle. The CI-V adapter
// lives in core/civ, which imports core/transport and not the other way
// round.
//
// It carries the whole cat.Dialect, not a bag of extracted funcs: both the
// write gate (d.AllowedCommand) and the session-init frame
// (d.BuildAISet(false)) come from that ONE value, which is the binding
// M9c-5 (E3) established and D2 preserves.
type catFraming struct {
	d cat.Dialect
}

// NewAccumulator returns a cat.FrameAccumulator: the ';'-terminated CAT
// shape, shared by every Yaesu dialect. max <= 0 selects
// cat.DefaultMaxFrame.
func (f catFraming) NewAccumulator(max int) Accumulator {
	return cat.NewFrameAccumulator(max)
}

// IsRejection reports whether frame is CAT's single unattributed NAK,
// "?;".
func (f catFraming) IsRejection(frame []byte) bool { return cat.IsRejection(frame) }

// Allow is the dialect's own outbound gate — an Engine built from this
// framing gates for the radio of the driver that built it and for no
// other.
func (f catFraming) Allow(frame []byte) bool { return f.d.AllowedCommand(frame) }

// InitSequence is the one frame a fresh CAT session establishes: AI0;,
// disabling Auto Information. Built by THIS framing's own dialect, the
// same value its gate came from, so the frame Init sends and the gate that
// judges it can never belong to different radios.
func (f catFraming) InitSequence() []Command {
	return []Command{f.d.BuildAISet(false)}
}

// DrainPolicy is CAT's: QuietPeriod of silence to call the line drained,
// and twice that as the absolute ceiling on any one drain. Both values are
// the ones Engine used before D2 — QuietPeriod for drainToQuietLocked's
// idle timer, 2*QuietPeriod for the bounded context the internal
// quarantine drains already ran under — so no CAT exchange's timing
// changes.
func (f catFraming) DrainPolicy() DrainPolicy {
	return DrainPolicy{IdleGap: QuietPeriod, Cap: 2 * QuietPeriod}
}

// NoteSent is a no-op for CAT. Yaesu's CAT link does not echo what the
// host writes — the accumulator sees only what the radio sends — so there
// is nothing to record. The hook exists for CI-V, whose bus and USB
// variants both echo, and whose accumulator drops the first received frame
// byte-equal to a noted sent one.
func (f catFraming) NoteSent([]byte) {}

// CATReadSpec builds the CommandSpec for one CAT READ: an answer whose
// frame starts with prefix and — when exactLen > 0 — is exactly exactLen
// bytes including the trailing ';'. retryReads is how many ADDITIONAL
// attempts Do may make after a timeout; a read is idempotent, so retrying
// one is safe (safety obligation 2).
//
// It is the sanctioned way for a CAT driver to build a read spec, and it
// exists because D2 retired the implicit ExpectPrefix=="" keying: the
// class is now explicit on every spec, and no wrapper may rewrite one
// later. The matcher itself is cat.PrefixLenMatcher — the CODEC's rule,
// held opaquely here — whose doc comment carries the full-address
// obligation for the EX family and the negative-space proof of what a bare
// prefix costs.
func CATReadSpec(prefix string, exactLen, retryReads int) CommandSpec {
	return CommandSpec{
		Class:      ClassRead,
		Match:      cat.PrefixLenMatcher(prefix, exactLen),
		RetryReads: retryReads,
	}
}

// CATWriteSpec builds the CommandSpec for one CAT WRITE: fire-and-forget,
// never retransmitted, quarantined afterwards whatever its outcome.
//
// This is what the zero CommandSpec used to mean, and the reason it no
// longer does. "Silence means it worked" is a real, load-bearing claim
// about the Yaesu reference — a Set gets no positive acknowledgement — and
// D2's judgement is that a claim of that weight should be WRITTEN, not
// inferred from an empty struct that could equally be an author who forgot
// to fill one in. CAT has no acknowledged write form, so there is no
// CATWriteWithAckSpec: ClassWriteWithAck exists for CI-V's FB.
func CATWriteSpec() CommandSpec {
	return CommandSpec{Class: ClassWrite}
}
