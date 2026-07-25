// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// Digest computes a hex-encoded SHA-256 digest over channels, canonicalised
// by sorting a copy by Slot (byte order) and then JSON-encoding the sorted
// slice. encoding/json's field order for a fixed struct type follows the
// struct's declaration order, not map iteration, so re-encoding the same
// sorted slice of Channel values always produces the same bytes.
//
// Digest identifies CONTENT, and content alone: it is a pure function of
// the Channel values passed in, independent of where they came from or
// when they were read. Two content-identical images digest EQUAL no
// matter how they were obtained — in particular, Digest by itself does
// NOT detect a reconnect or a re-read: a reconnect/re-read that happens
// to read back identical data produces the SAME digest, not a different
// one. What Digest reliably detects is any EDIT: changing so much as one
// field of one channel changes the digest (RadioInfo.BaselineDigest
// records the Digest of the Channels read from a radio at read time
// precisely so a later comparison can catch that).
//
// Binding a send confirmation to session/device identity — the CAT ID
// answered by the currently-connected radio, its USB serial, a read
// generation counter, or similar — is a separate concern this function
// does not, and cannot, address on its own; that binding is the
// transport/clone layer's responsibility (see DiffResult.CandidateDigest's
// doc comment for how a sender is expected to combine the two: content
// identity from Digest, plus its own session/device identity check,
// immediately before transmission).
//
// Digest is independent of channels' input ordering (two slices holding
// the same Channel values in different orders produce the same digest)
// and sensitive to every field of every channel: changing any field of
// any channel changes the digest, and an empty slot (Data == nil) never
// digests the same as a populated one at the same Slot.
func Digest(channels []Channel) string {
	sorted := make([]Channel, len(channels))
	copy(sorted, channels)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Slot < sorted[j].Slot
	})

	b, err := json.Marshal(sorted)
	if err != nil {
		// Channel/ChannelData hold only strings, bools, numbers and a
		// pointer to a like-shaped struct — every field is fully
		// value-typed, and nothing value-typed can make json.Marshal
		// fail (no channels, funcs, or cycles). This is unreachable in
		// practice today; a panic here would only ever fire if that
		// invariant were broken by a FUTURE field addition that is not
		// value-typed (e.g. a func, channel, or a type embedding one),
		// which would also make this whole function genuinely fallible —
		// exactly when we want a loud failure here rather than a
		// silently wrong digest reaching a send confirmation.
		panic("codeplug: Digest: unexpected json.Marshal failure: " + err.Error())
	}

	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
