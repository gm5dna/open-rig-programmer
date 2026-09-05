// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// PrefixLenMatcher returns the ANSWER MATCHER for one CAT read: a
// predicate reporting whether an arriving frame is the answer to the
// command the caller is about to send. It is the CAT codec's contribution
// to a transport CommandSpec — the transport layer holds the predicate and
// never looks inside it, so the matching RULE belongs to the protocol that
// defines it rather than to the engine that applies it (M9d-2/D2: "answer
// matching moves into the spec").
//
// The rule is exactly the one core/transport applied inline before D2, so
// no CAT exchange changes shape: the frame must be at least as long as
// prefix and start with it, and — when exactLen > 0 — must be exactly
// exactLen bytes including the trailing ';'. exactLen <= 0 means variable
// length (an MT answer, whose tag is 0-12 bytes), where only the prefix is
// checked.
//
// PREFIX WIDTH IS THE CALLER'S OBLIGATION, and it is a safety one. For a
// command family whose answer frames share a short prefix across MANY
// distinct addresses — EX (MENU) is the case in point, where every one of
// the 296 Table 2 addresses answers with a frame starting "EX" — prefix
// MUST include the full address, e.g. "EX"+d.EXWire(addr), never the bare
// command name "EX". The returned matcher only compares
// frame[:len(prefix)]; a bare "EX" would let it correlate ANY EX answer —
// a different address's own reply still in flight, or an unsolicited
// AI-pushed EX frame — as this read's answer, silently returning the wrong
// address's data. See core/transport/engine_ex_test.go
// (TestEngine_EXRead_PrefixOnlySpec_DemonstratesWrongAddressHazard) for
// the negative-space proof, and
// TestEngine_EXRead_WrongAddressInjectedFirst_NotConsumed for the same
// injection surviving correctly once the prefix carries the full address.
// This is a convention callers follow, not something this function can
// enforce: it has no address-shaped parameter, and the prefix comparison
// is otherwise exactly what every other single-address command family
// (MR, MT, ...) already relies on.
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
