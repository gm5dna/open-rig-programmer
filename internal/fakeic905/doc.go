// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic905 simulates an Icom IC-905's binary CI-V behaviour over an
// in-memory connection (Radio.Port()). It is the test double the IC-905 tier's
// own layers run against — the transport engine, the driver, and any --fake or
// demo path that reaches a rig — the role internal/fakeradio plays for the
// FT-710 and internal/fakedx101 for the FTdx101 pair.
//
// # THE HARD RULE: NOTHING project-internal
//
// fakeic905 imports NOTHING from github.com/gm5dna/open-rig-programmer/. Not
// core/civ. Not core/civ/ic905. Not core/driver/ic905. Not core/spec, not
// core/codeplug, not internal/fakeradio, not internal/fakedx10, not
// internal/fakedx101. Not a constant, not a type, not a test helper. Standard
// library only, in every non-test file, in this directory AND every directory
// beneath it.
//
// IT IS A SIBLING OF THE DIALECT, NOT A REFACTOR OF IT. Every framing byte,
// every rejection rule and every offset named below is re-derived here from the
// IC-905 CI-V REFERENCE GUIDE's own printed diagrams, by way of the two
// quarantined artefacts PROVENANCE.md names and the frame diagrams the task
// brief quoted. Where this package and core/civ/ic905 agree, they agree because
// two readings of one document landed in the same place — which is evidence.
// Where one of them imported the other, agreement would be a tautology, and the
// evidence would be worth nothing.
//
// The reasoning is internal/fakeradio's, verbatim, and it is worth restating
// because it is the entire point of the rule: if this fake reused the
// production codec, a systematic bug in that codec — an off-by-one in a field
// offset, a validation rule subtly wrong, a length fingerprint mistyped — would
// be applied identically on both sides of every "send a command, check the
// reply" test the project runs. The bug would never surface. The fake would
// misbehave in exactly the way the buggy codec expects, and every end-to-end
// test would pass anyway. Two independent implementations of one protocol,
// checked against each other and against expectations recomputed BY HAND in
// tests — never by calling this package's own builders — is what makes that
// class of bug visible.
//
// TestNoCoreImports (imports_test.go) enforces it with a go/parser scan that
// WALKS SUBDIRECTORIES, with vacuity guards and its own red proof. That file is
// internal/fakedx101's, COPIED — copied rather than imported, because importing
// the thing that enforces "import nothing" would break the rule it enforces.
//
// # What this fake does
//
// It speaks the two commands this tier sends and refuses everything else:
//
//   - 19 00, "Read transceiver ID", answered with a configurable token whose
//     value this package asserts nothing about (register entry 4).
//   - 1A 00, "Memory contents", symmetric: four printed address bytes then a
//     record, in both directions.
//
// It STORES AND RETURNS RAW BYTES AND NEVER INTERPRETS THEM. The only question
// it ever asks about a record is how long it is, and even that question is
// asked relatively: a set whose record length differs from the length this fake
// already HOLDS for that channel is refused. The lengths {64, 65} are the
// DRIVER's fingerprint, not this fake's rule — a Radio will hold a 64-byte
// record, a 65-byte one, a 39-byte one or a 7-byte one with equal composure,
// which is what the fingerprint and wrong-sibling tests need of it.
//
// A record containing FE or FD is stored and answered back unescaped, which
// breaks the framing of the answer carrying it. That is deliberate: the fake
// holds no opinion about record content, and a consumer seeding such a record
// is seeding one no radio could send.
//
// # Rejection, and the difference between NG and silence
//
//	unknown command or sub-command, addressed to AC   -> FE FE E0 AC FA FD
//	malformed frame, addressed to AC                  -> FE FE E0 AC FA FD
//	over-length run                                   -> FE FE E0 AC FA FD
//	read of an unoccupied channel                     -> FE FE E0 AC FA FD
//	set at a length the fake does not hold            -> FE FE E0 AC FA FD
//	accepted set                                      -> FE FE E0 AC FB FD
//	a frame addressed ANYWHERE BUT AC                 -> nothing at all
//
// The last line is the one that matters most. A radio at a different address
// never hears the frame, and the controller times out; a fake that answered NG
// instead would make the driver's timeout branch untestable, because nothing
// would ever fail to answer.
//
// # THE ASSUMED REGISTER
//
// Everything this package does that is NOT printed in the artefacts it read,
// with the lift that would retire each entry. A lift is per model and per
// firmware: nothing observed of any other radio, and nothing this fake does,
// is evidence about an IC-905.
//
//  1. A `1A 00` READ OF AN UNOCCUPIED CHANNEL IS ANSWERED NG. Nothing in either
//     artefact says what a real IC-905 does with a read of an empty channel;
//     the printed material describes the record's shape and a clear command,
//     and stops there. NG is this fake's choice, made because a driver needs
//     SOME distinguishable answer for "nothing there".
//     LIFT ic905-R-14: a read of a known-empty channel on a real IC-905, with
//     the reply captured.
//
//  2. THE TRANSCEIVE BROADCAST FORM (to = 00). The reference guide's data-format
//     page prints four complete frames and NONE of them is a broadcast. The
//     `to = 00` spelling this fake emits under WithTransceiveBroadcasts is the
//     form the tier's address filter is designed for; no IC-905 has been
//     observed emitting anything.
//     LIFT ic905-R-12: a capture of an IC-905 with transceive enabled,
//     recording what its unsolicited frames actually look like.
//
//  3. THE CONTROLLER-ADDRESSED FLOOD (to = E0). Same standing as entry 2, and
//     recorded separately because it is a separate claim: that a radio might
//     put frames addressed to the controller on the wire faster than the
//     controller consumes them. The fake emits the form the tier's drain is
//     designed for. Nothing has been observed.
//     LIFT: the same capture as ic905-R-12, read for controller-addressed
//     traffic.
//
//  4. THE DEFAULT IDENTITY TOKEN'S VALUE (DE AD). The wire facts this package
//     was built from give the command — 19 00, request carrying no data bytes —
//     and say NOTHING about what comes back. DE AD was chosen precisely so that
//     no reader could mistake it for a fact about an IC-905, and
//     WithIdentityToken exists so a consumer can pin a different one and prove
//     its driver records whatever it gets rather than matching a value.
//     LIFT: a 19 00 exchange with a real IC-905, with the answer captured.
//
//  5. THE DEFAULT IMAGE — WHICH CHANNELS ARE OCCUPIED, AND WHAT THEY HOLD. Ten
//     channels in group 0, each holding sixty-four zero bytes. The reference
//     prints each field's permitted VALUES and never a shipped default, so
//     there is nothing to source a factory record from and every byte of this
//     one is invented. Zeros were chosen because they invent the least; they
//     are not a claim that any IC-905 answers zeros, and a consumer that needs
//     content seeds it with WithRecord. This matters beyond the test suite:
//     anything that renders a fake rig's memories to a user shows these bytes,
//     and a user must not read them as what an IC-905 ships with.
//     LIFT: a full memory read of a factory-reset IC-905.
//
//  6. THE TWO ADDRESS FIELDS READ AS PACKED BCD. Both address rows of
//     ic905-transcription-b.csv record their encoding column as `unstated`: the
//     page prints permitted values and never says how to read them. The values
//     are the whole of the evidence, and they read as BCD in both fields —
//     `00 00 ~ 00 99: 00 ~ 99`, and `00 10, 00 11: 10G C1, C2` where a binary
//     field would print `00 0A`. On that reading the printed call-channel
//     group `01 00` is group 100, which is how WithRecord addresses it.
//     LIFT: a statement of the encoding elsewhere in the guide (the pages this
//     project's artefacts covered do not carry one), or a capture.
//
//  7. SILENCE FOR A FRAME ADDRESSED ELSEWHERE. Both printed frame diagrams
//     carry a `to` byte, and an address byte exists to single out one radio, so
//     a radio ignoring what is not addressed to it is a very short inference —
//     but it IS an inference, because no printed line says "and otherwise it
//     says nothing".
//     LIFT: a frame addressed to another CI-V address put to a real IC-905 on a
//     shared bus, with the silence observed.
//
//  8. REFUSING THE PRINTED CLEAR FORM. ic905-transcription-b.csv's D2 block
//     prints the clear command — four address bytes then a single FF, with
//     nothing after it — so a real IC-905 evidently honours it. This fake
//     refuses it because THIS TIER DOES NOT SEND IT, and a fake that accepted
//     traffic the tier never emits would be simulating a radio nobody is
//     driving.
//     LIFT: the day this tier sends a clear, this entry goes and the behaviour
//     changes with it.
//
//  9. REFUSING 1A 01, 1A 02, 1A 05, 09, 0A, 0B AND A0. Same standing as entry
//     8, and at least two of them are demonstrably real commands: 1A 01 is the
//     "Band stacking register" whose own diagram sits on PDF page 20, and 1A 05
//     heads the set-mode command table on PDF page 9. Refusing them is tier
//     policy, not a fact about the radio.
//     LIFT: the tier learning to send any of them.
//
//  10. THE MAXIMUM FRAME LENGTH, AND NG FOR AN OVER-LENGTH RUN. The document
//     states no frame-length limit. The cap (maxBodyBytes) is a property of a
//     reader that must not grow without bound on a line that has come up
//     mid-frame; NG is the honest answer to "something arrived that could not
//     be a frame", but nothing says a radio sends one.
//     LIFT: an over-long run put to a real IC-905, with its response recorded.
//
// # What is NOT in this register
//
// The framing itself — FE FE, FD, AC, E0, FB, FA, the two frame orders, and
// `1A 00`'s symmetry — is PRINTED, on the reference guide's data-format page
// (PDF p.3, folio 2) and its command table (PDF p.6, folio 5). So is the
// record's geometry: 68 bytes, indices 1-68, the group and channel fields
// first. Those are facts, cited in parser.go and state.go where they are used,
// and they do not belong in a register of assumptions.
//
// The record-length rejection rule is not an assumption of this package either.
// It was supplied as a wire fact, stated without the layout: this model's
// records come in exactly two lengths, and a set at a length the fake does not
// hold for that channel is answered FA.
//
// # HARDWARE STATUS: UNVERIFIED
//
// NO IC-905 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Every byte in this
// package descends from rasterised pages of one PDF, read by eye, by agents who
// never opened this repository. Nothing here has been put to a radio, and until
// something is, every entry in the register above stands.
package fakeic905
