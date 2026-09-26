// SPDX-License-Identifier: GPL-3.0-or-later

// Package clonewire is the receive-side transport for the clone-mode
// radios registered by the 2026-09-25 clone-mode READ milestone
// (FT-817/FT-817ND/FT-818, FT-857/FT-857D, FT-897/FT-897D): a whole-image
// transfer the radio starts unprompted, once an operator presses its own
// front-panel button sequence — never a query/reply exchange
// (spec.md §Context, §Codec placement).
//
// It is a SIBLING of core/cat, core/civ, core/kw and core/bincat, not a
// relative: those four are all request/reply command protocols, and none
// of their framing applies here — there is no command to send, no
// terminator, no address byte. It is also NOT a core/transport.Engine
// Framing plugged into the existing engine (spec.md §Codec placement):
// Engine.Do always transmits before it will read, and Accumulator.Push has
// no idle-time callback, so neither fits an unsolicited, operator-triggered
// transfer. This package uses transport.Port directly — the same
// io.ReadWriteCloser OpenSerial returns — and owns its own bounded receive
// loop on top of it.
//
// # ACK-only outbound (spec.md, Decisions item 5)
//
// The read-only ruling holds: this package never sends a memory-content
// write frame, and never will. But CHIRP's own FT-817 driver shows the PC
// end is not purely passive either — it sends 0x06 (ACK) back to the radio
// after every block, and the transfer stalls without it. Stuart's
// 25/09/2026 answer to Q1 permits exactly this: a per-block ACK byte,
// carrying no memory content and deciding nothing about what the radio
// does, sent only where the matched Profile's block schedule says one is
// expected. Every other outbound byte stays refused — Receive never writes
// anything but that single ACK byte, and only at the offsets Profile.
// AckExpected/Block.Ack name.
//
// # Arm is its own step (spec.md §Read model, point 1)
//
// Arming and receiving are two separate calls, not one blocking
// ReadImage(ctx, port, profiles): Arm starts the deadline clocks on the
// already-open port and returns as soon as they are live, via Reception.
// Armed(). Only once a caller has observed that signal should it print the
// "put the radio into clone-send mode now" prompt (Phase 4's job) — arming
// after the prompt risks losing a fast-starting radio's opening bytes.
//
// # Whole-image refusal (spec.md §Read model, point 5)
//
// Any deadline expiry, a checksum failure, a short image or trailing bytes
// beyond a Profile's block schedule all refuse the WHOLE image — there is
// no partial parse. A clone-mode image has no per-region checksum this
// project can verify without a hardware capture, so a partially accepted
// image risks silently wrong channel data with no way to tell it apart
// from a genuine radio state.
//
// # Candidate profiles, not one Profile (spec.md §Identity probe)
//
// No clone-mode radio in this family sends a model-identifying byte.
// Receive is always given a candidate SET: total received image length is
// checked against every candidate's ImageLen — zero matches is
// ErrImageIncompatible, more than one match is ErrImageAmbiguous, exactly
// one proceeds to a parse. This is a compatibility check, not an identity
// proof.
//
// All candidates offered to one Arm/Receive call are assumed to share the
// SAME physical wire framing (baud, parity, checksum algorithm, ACK
// timing) — Receive drives the live read from candidates[0]'s
// BlockSchedule and deadlines, and consults every candidate only for the
// post-receipt length match. This mirrors core/bincat's own precedent
// (FT-890/FT-900: "ONE GO TYPE, MANY VALUES" differing only in slot count
// and full-dump length under identical framing): every real family this
// milestone builds (Phase 2) is CHIRP-documented as one driver per family
// with several image lengths, not several distinct wire protocols. If a
// future family genuinely needs candidates with differing framing, that is
// new work for Receive's internals, not a change to this package's public
// signatures.
//
// # No real profiles here
//
// This phase ships the primitive only: Arm, Reception.Receive, the
// Profile/Block shapes, and the candidate-matching errors. Every field on
// Profile is exercised by this package's own tests against a synthetic
// fixture profile — no radio's real baud, block schedule or record layout
// is set here. Those are Phase 2's job, informed-by CHIRP per family.
//
// # Phase 2: real profiles, CHIRP provenance pin
//
// FT817Family (FT-817/FT-817ND/FT-818, five image lengths) and the
// FT-857/FT-857D and FT-897/FT-897D profiles are informed-by CHIRP's
// chirp/drivers/{ft817,ft818,ft857}.py and chirp/drivers/yaesu_clone.py,
// pinned at commit e7347e6a66ef8f9edb50e3e534510c2d3ae6b329
// (github.com/kk7ds/chirp). CHIRP has no separate ft897.py: its own
// FT857Radio class covers "FT-857/897" as one MODEL, identical _memsize
// and block schedule for both radios — this project still models FT-857/
// FT-857D and FT-897/FT-897D as separate Profile values (spec.md
// §Identity probe wants one Profile per named model), but since CHIRP
// itself supplies no byte that tells those apart, offering them together
// to one Arm/Receive call correctly returns ErrImageAmbiguous — this is
// the spec-required outcome for "nothing distinguishes them", not a bug.
// FT-857/FT-857D/FT-897/FT-897D's Baud is ASSUMED, not clone-confirmed,
// informed-by manual CAT RATE menu 019, no HW capture (Stuart, 25/09/2026)
// — see each Profile's own comment.
//
// # Block.HeaderBytes/TrailerBytes (extended internally, no API change)
//
// Every real family above frames its wire blocks as CHIRP's
// yaesu_clone.py does: [1-byte block number][payload][1-byte checksum].
// Checksum must validate that whole framed chunk, but Image.Raw must hold
// only the concatenated payload bytes (what a Profile's ImageLen/
// RecordOffset/RecordWidth are stated against, and what CHIRP's own mmap
// contains). Block gained two fields, HeaderBytes/TrailerBytes, to strip
// that framing before the content reaches Raw; Receive's internal
// accumulation step honours them. This is exactly the "genuinely
// divergent wire schedule" case flagged above ("new work inside Receive,
// not a signature change") — Arm, Receive and ParseImage's signatures are
// unchanged; only the Block struct (already documented as Phase 2's to
// fill in) gained two zero-defaulting fields, and Phase 1's own tests
// (which never set them) are unaffected.
package clonewire
