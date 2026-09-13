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

// MRAnswerMatcher returns the ANSWER MATCHER for one MR read: the same
// contract PrefixLenMatcher serves, with the ANSWERED SLOT compared against
// the slot the outstanding read named.
//
// WHY MR NEEDS ITS OWN AND THE OTHER FRAMES DO NOT. PrefixLenMatcher
// correlates on a prefix and a length, which is enough for every fixed
// singleton this milestone reads — one ID answer, one FV, one TY, one MC —
// and enough for EX only because the caller is REQUIRED to put the whole
// address in the prefix (see above). MR is the third case: every one of a
// radio's memory answers is 50 bytes and starts "MR", and the channel number
// sits at P2/P3 rather than immediately after the command name, so no prefix
// a caller can spell separates one channel's answer from another's. The
// sequence that exploits it is ordinary rather than exotic: a read of 007
// times out, the engine quarantines the port, a read of 008 goes out, and
// 007's very late answer arrives while 008's read is waiting. Under a
// prefix-and-length matcher that frame IS 008's answer, and ParseMRAnswer
// then returns a record whose Slot says 007 —
// TestMRAnswerMatcher_ALateAnswerIsNeverTheNextReadsAnswer drives exactly
// that through the real engine.
//
// IT COMPARES THE SLOT AND NOT P1, and the division is deliberate. P1 is the
// half of a section-defined channel (590:1529-1531), and a driver's own read
// path compares it against the half it asked for; a matcher that also
// refused on P1 would make an answer for the right channel's wrong half look
// like no answer at all — a timeout where the driver has a precise,
// reportable mismatch. The slot is the correlation key; the half is the
// driver's check on a frame already correlated.
//
// THE COMPARISON IS OF THE SLOT, NOT OF THE BYTES. A 590 answers a channel
// below 100 with a SPACE in P2 (590:1332-1337, inherited by MR under A10)
// while this codec's own read spells it '0', so comparing P2/P3 literally
// against the read frame would refuse nearly every genuine answer. Decoding
// through parseSlot — the same method the record parser and the outbound
// gate use — is what makes the two spellings one slot, and keeps the
// convention read from one place.
//
// IT IS A CORRELATION PREDICATE AND NOT A PARSE. Everything past the slot —
// the printed-fixed bytes, the mode nibble, the field domains — stays
// ParseMRAnswer's, so a corrupt answer TO THIS READ is still delivered and
// still refused with a message naming what was wrong with it, rather than
// silently missing its match and being reported as a timeout.
//
// CONTRACT ON frame: as PrefixLenMatcher's — the returned matcher only READS
// frame and never retains it.
func (l Layout) MRAnswerMatcher(s Slot) func(frame []byte) bool {
	return func(frame []byte) bool {
		if len(frame) != int(l.recordLen) {
			return false
		}
		if frame[recPrefixOff] != 'M' || frame[recPrefixOff+1] != 'R' {
			return false
		}
		got, err := l.parseSlot("MR answer", frame, frame[recP1Off])
		if err != nil {
			return false
		}
		return got.number == s.number
	}
}
