// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ExactLengthMatcher returns the Match a Status Update read waits on.
//
// IT ACCEPTS ANY FRAME, and that is correct rather than lazy: the
// accumulator itself (framing.go) already enforces the exact length
// NoteSent recorded for this exchange — nothing shorter can complete, and
// this family's link is point-to-point with no bus and nothing else that
// could deliver an unrelated frame, unlike CI-V's address-checked matchers
// which exist specifically to reject another station's traffic. A driver
// that needs to inspect the answer's CONTENT does so itself after Do
// returns; this matcher's only job is "a complete reply arrived", which
// the accumulator has already decided by the time Match is ever called.
func ExactLengthMatcher() func(frame []byte) bool {
	return func(frame []byte) bool { return true }
}

// ReadSpec builds the CommandSpec for one Status Update read: Class,
// Match and Timeout stated explicitly, mirroring civ.CIVReadSpec's role
// for this family — D2's rule that no spec relies on an implicit default
// class applies here exactly as it does to CI-V.
//
// retryReads is safe for civ.CIVReadSpec's own reason: a read of a memory
// or VFO record changes nothing on the radio, so a lost or corrupted
// reply may be resent.
func ReadSpec(timeout time.Duration, retryReads int) transport.CommandSpec {
	return transport.CommandSpec{
		Class:      transport.ClassRead,
		Match:      ExactLengthMatcher(),
		Timeout:    timeout,
		RetryReads: retryReads,
	}
}

// WriteSpec builds the CommandSpec for one fire-and-forget mutation —
// every opcode in this family's write choreography (spec.md §Write
// model): no acknowledgement and no NAK, so ClassWrite is this family's
// ONLY write class. There is no ClassWriteWithAck builder here, unlike
// CI-V's acknowledged memory set: this family's manuals document no
// acknowledgement frame of any kind.
func WriteSpec(errorWindow time.Duration) transport.CommandSpec {
	return transport.CommandSpec{
		Class:       transport.ClassWrite,
		ErrorWindow: errorWindow,
	}
}
