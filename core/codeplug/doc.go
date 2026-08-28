// SPDX-License-Identifier: GPL-3.0-or-later

// Package codeplug is the offline in-memory model of a radio's memory
// image, and its on-disk JSON file format. It imports core/spec and the
// standard library only — never core/cat — so the model and its
// persistence format stay independent of any particular radio's wire
// protocol; a radio driver translates between the two.
//
// Three rules make this package's model safe to build the rest of the
// project on:
//
//   - A channel is either empty or populated, never both. Channel.Data ==
//     nil is the SOLE discriminator (see Channel.Empty): there is no
//     separate "in use" flag that could disagree with the data, so a
//     contradictory state ("has data but marked empty") is simply not
//     representable.
//
//   - "We know this value" is explicit, and distinct from "the CAT
//     protocol cannot reach this value" — never a bare nil standing in
//     for either. ToneField and BoolField each carry a FieldState (Known,
//     Unknown, or Unavailable) alongside their Value. The write rule this
//     exists to enforce: only a Known value is ever sent to a radio.
//     Unknown and Unavailable both mean "preserve whatever the radio
//     currently has", and a write path must never synthesise or guess a
//     value for either.
//
//   - Files are written atomically and durably (Save), and carry both a
//     schema version (Codeplug.Schema, checked against CurrentSchema by
//     Load — and, since the Icom tier, CHOSEN by Save as the lowest
//     schema that can represent the content, so a file gains a version
//     only when its content needs one; see CurrentSchema and schemaFor)
//     and a baseline digest (RadioInfo.BaselineDigest, computed by
//     Digest) so a send confirmation can be bound to the exact radio
//     image it was computed from. Any reconnect, re-read, or edit that
//     changes so much as one field produces a different digest, which
//     lets a later send/clone path detect that the assumed baseline no
//     longer holds.
package codeplug
