// SPDX-License-Identifier: GPL-3.0-or-later

package kw

// PrefixLenMatcher returns the ANSWER MATCHER for one Kenwood read: a
// predicate reporting whether an arriving frame is the answer to the
// command the caller is about to send. It is this codec's contribution to a
// transport CommandSpec — the transport layer holds the predicate and never
// looks inside it, so the matching RULE belongs to the protocol that
// defines it rather than to the engine that applies it.
//
// COPIED FROM core/cat/matcher.go's rule, not imported (the fence,
// imports_test.go). The rule: the frame must be at least as long as prefix
// and start with it, and — when exactLen > 0 — must be exactly exactLen
// bytes including the trailing ';'.
//
// exactLen <= 0 MEANS VARIABLE LENGTH, and on this pair that branch has
// exactly ONE user: the EX answer. Every other frame either radio sends is
// fixed — ID 6, FV 7, TY 6, MC 6, MR 50 — and the EX answer's P5 is
// declared "String of alphanumeric characters for the Menu setting
// (variable length)" (590:555-556) and "A string of characters (Variable
// length). Normally 1-digit for the TS-480. Menu No. 32, 35 and 48 ~ 52 use
// 2-digit parameters" (480:409-411), with no printed ceiling — recorded as
// A19. That negative fact is stated in doc.go too, because it is what makes
// this branch's presence a design commitment rather than spare generality.
// (On the Yaesu side the same branch serves MT, a command this family does
// not have at all.)
//
// PREFIX WIDTH IS THE CALLER'S OBLIGATION, and it is a safety one. For the
// EX family it is the LOAD-BEARING half of this function. Every one of the
// TS-590SG's 100 menu addresses, the TS-590S's 88 and the TS-480's 61
// answers with a frame starting "EX" (590:543-544, 480:401), so prefix MUST
// carry the full three-digit address — "EX" + the address field, never the
// bare command name "EX". The returned matcher only compares
// frame[:len(prefix)]; a bare "EX" would let it correlate ANY EX answer —
// a different address's own reply still in flight — as this read's answer,
// silently returning the wrong address's data.
// TestPrefixLenMatcher_FullAddressObligation is the negative-space proof:
// the bare prefix accepts a foreign address's answer, the full-address
// prefix refuses it and still accepts our own. This is a convention callers
// follow, not something this function can enforce: it has no
// address-shaped parameter.
//
// CONTRACT ON frame: the returned matcher only READS frame and never
// retains it — it is handed the engine's own live receive buffer.
func PrefixLenMatcher(prefix string, exactLen int) func(frame []byte) bool {
	return func(frame []byte) bool {
		if len(frame) < len(prefix) {
			return false
		}
		if string(frame[:len(prefix)]) != prefix {
			return false
		}
		if exactLen > 0 && len(frame) != exactLen {
			return false
		}
		return true
	}
}
