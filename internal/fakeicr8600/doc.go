// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeicr8600 simulates an Icom IC-R8600's binary CI-V behaviour over
// an in-memory connection (Radio.Port()). It is the test double the IC-R8600's
// own layers run against — the transport engine, the driver, and any --fake or
// demo path that reaches a rig — the role internal/fakeradio plays for the
// FT-710, internal/fakedx101 for the FTdx101 pair and internal/fakeic905 for
// the IC-905.
//
// # THE HARD RULE: NOTHING project-internal
//
// fakeicr8600 imports NOTHING from github.com/gm5dna/open-rig-programmer/. Not
// core/civ. Not core/civ/icr8600. Not core/driver/icr8600. Not core/spec, not
// core/codeplug, not internal/fakeradio, not internal/fakeic905, not
// internal/fakeic7610. Not a constant, not a type, not a test helper. Standard
// library only, in every non-test file, in this directory AND every directory
// beneath it.
//
// IT IS A SIBLING OF THE DIALECT, NOT A REFACTOR OF IT. Every framing byte,
// every offset, every mode code and every rejection rule below was re-derived
// here from the IC-R8600 CI-V REFERENCE GUIDE, rev A7375-2EX-3a, by way of the
// quarantined artefacts in core/civ/icr8600/testdata/ — the semantic
// transcription (IC-R8600-transcription-b.csv/.md), the geometry witness
// (IC-R8600-geometry-witness.csv/.md) and the golden vectors' provenance
// (IC-R8600-golden-provenance.md). NO SOURCE FILE OF core/civ/icr8600 OR
// core/driver/icr8600 WAS READ WHILE THIS PACKAGE WAS WRITTEN. Where this
// package and the profile agree, they agree because two readings of one
// document landed in the same place — which is evidence. Where one of them
// imported the other, agreement would be a tautology, and the evidence would be
// worth nothing.
//
// The reasoning is internal/fakeradio's, and it is worth restating because it
// is the entire point of the rule: if this fake reused the production codec, a
// systematic bug in that codec — an off-by-one in a field offset, a mode code
// mistyped, a length fingerprint wrong — would be applied identically on both
// sides of every "send a command, check the reply" test the project runs. The
// bug would never surface. Two independent implementations of one protocol,
// checked against each other and against expectations recomputed BY HAND in
// tests — never by calling this package's own tables — is what makes that class
// of bug visible.
//
// TestNoCoreImports (imports_test.go) enforces it with a go/parser scan that
// WALKS SUBDIRECTORIES, with vacuity guards and its own red proof. That file is
// internal/fakeic905's, COPIED — copied rather than imported, because importing
// the thing that enforces "import nothing" would break the rule it enforces.
//
// # What this fake does
//
// It speaks the two commands this tier sends and refuses everything else:
//
//   - 19 00, "Read the receiver ID", answered with a configurable token whose
//     value this package asserts nothing about (register entry 1).
//   - 1A 00, "Send/read memory channel contents", symmetric: four printed
//     address bytes then a record, in both directions.
//
// # The one byte it reads, and why
//
// Its sibling fakes hold records as opaque bytes and ask only how long they
// are, because on those radios a length names a layout. IT CANNOT HERE. The
// IC-R8600's record is a 37-byte record-only head followed by a MODE-SELECTED
// tail — none for the eleven analogue modes, +2 D-STAR, +4 P25, +6 NXDN, +7 FM,
// +7 DCR, +8 dPMR — so the accepted record-only set is {37, 39, 41, 43, 44, 45},
// six values from seven layouts, and FM and DCR are BOTH 44 bytes with
// different contents. Length names nothing on its own.
//
// So this fake reads exactly ONE byte of a record: printed index (11), the
// receiving mode, at record offset 6. It reads it to choose a layout, and for
// nothing else. No other byte of any record is interpreted, compared, validated
// or repaired, and a stored record is served back exactly as it arrived.
//
// # Rejection, and the difference between NG and silence
//
//	unknown command or sub-command, addressed to 96   -> FE FE E0 96 FA FD
//	malformed frame, addressed to 96                  -> FE FE E0 96 FA FD
//	over-length run                                   -> FE FE E0 96 FA FD
//	read of an unoccupied channel                     -> FE FE E0 96 FA FD
//	set whose mode byte names no layout               -> FE FE E0 96 FA FD
//	set whose length is not its mode's layout's       -> FE FE E0 96 FA FD
//	the printed clear form (address + FF)             -> FE FE E0 96 FA FD
//	accepted set                                      -> FE FE E0 96 FB FD
//	a frame addressed ANYWHERE BUT 96                 -> nothing at all
//
// The last line is the one that matters most. A receiver at a different address
// never hears the frame, and the controller times out; a fake that answered NG
// instead would make the driver's timeout branch untestable, because nothing
// would ever fail to answer.
//
// There are no textual rejection messages: the guide gives one NG code, FA,
// with no reason field, so every refusal above is the same six bytes. What a
// consumer can tell apart is WHICH FRAMES ARRIVED (Frames) and WHAT THE FAKE
// STILL HOLDS (Record) — a refused set stores nothing and changes nothing.
//
// # What this fake will NOT do
//
//   - It ships no erase. The clear form IS printed — PDF p.15 (folio 14),
//     "Command 1A 00 clears a memory channel by sending the command in the
//     following format" — and this tier admits no erase builder, so the fake
//     refuses it (entry 7). Unlike the Yaesu models, the wire form exists.
//   - It invents no scan-group encoding. Groups 0100 (Auto Write), 0101 (Scan
//     Skip) and 0102 (Programmable Scan Edge) are printed, and the A/B-suffixed
//     scan-edge channel numbers' wire encoding is not; the fake seeds nothing
//     there and guesses nothing.
//   - It fills in no short set by default (entry 11), and it guesses no mode
//     code (entry 2): a code the printed table does not list selects nothing.
//
// # THE ASSUMED REGISTER
//
// Everything this package does that is NOT printed in the document it was
// derived from, with the lift that would retire each entry. A lift is per model
// and per firmware: nothing observed of any other radio, and nothing this fake
// does, is evidence about an IC-R8600. Where the model's own matrix
// (docs/superpowers/icom-matrices/icr8600-capability-matrix.md) already carries
// an entry for the same open point, its id is named — this register is the
// FAKE's, and it does not replace the profile's or the driver's.
//
//  1. THE DEFAULT IDENTITY TOKEN'S VALUE (DE AD). The command table on PDF p.5
//     (folio 4) prints "19 / 00 / <blank Data cell> / Read the receiver ID":
//     the request's emptiness is documented and the ANSWER'S VALUE is printed
//     nowhere in the guide. DE AD was chosen precisely so that no reader could
//     mistake it for a fact, and WithIDToken exists so a consumer can pin a
//     different one and prove its driver records whatever it gets rather than
//     matching a value. Matrix entry: icr8600-id-token.
//     LIFT (Stage R): send FE FE 96 E0 19 00 FD to a real IC-R8600 and record
//     the answer's data bytes.
//
//  2. THE MODE WIRE CODES. PDF p.10 (folio 9)'s "(1) Receiving mode" table
//     prints eighteen two-character codes — 00-08, 11, 14-21 — and NEVER SAYS
//     whether a code is a packed-BCD byte or a binary number. Under BCD "21" is
//     0x21; under binary it is 0x15. This package takes the BCD reading, which
//     is what every other numeric field in the guide uses; "natural" is not
//     evidence. A code outside the eighteen selects no layout and is refused
//     rather than guessed at. Matrix entry: icr8600-mode-wire-codes.
//     LIFT (Stage R): set a real IC-R8600 to DCR, read the mode with command
//     04, and record the raw byte; repeat for FM and D-STAR.
//
//  3. THE TRANSCEIVE BROADCAST FORM (to = 00). The data-format page draws two
//     `to` values, 96 and E0, and NO broadcast frame anywhere. The 00 spelling
//     WithTransceiveBroadcasts emits is the form the tier's address filter is
//     designed for; no IC-R8600 has been observed emitting anything. The
//     frame's data is five zero bytes and claims nothing. Matrix entry:
//     icr8600-broadcast-form.
//     LIFT (Stage R): with transceive ON, capture an unsolicited frame from a
//     real IC-R8600 and record its `to` byte.
//
//  4. THE READ-REQUEST FORM. The guide declares 1A 00 a send/read pair (the
//     command table's asterisk, expanded on PDF p.9 as "*(Asterisk) Send/read
//     data") and DRAWS THE READ DIRECTION'S DATA AREA NOWHERE. "The four
//     address bytes, and then stop" is this package's reading. The only
//     partial data area the guide does print is the clear form, which is a
//     write. Matrix entry: icr8600-read-request-form.
//     LIFT (Stage R): send FE FE 96 E0 1A 00 00 00 00 01 FD to a real
//     IC-R8600 and record whether a record answer follows.
//
//  5. A READ OF AN UNOCCUPIED CHANNEL IS ANSWERED NG — the default. Nothing in
//     the guide addresses the read reply for an empty channel; PDF p.3 defines
//     FA only as the generic "NG code (fixed)". NG is this fake's choice, made
//     because a driver needs SOME distinguishable answer for "nothing there".
//     Matrix entry: icr8600-empty-reply-fa.
//     LIFT (Stage R): on a factory-fresh IC-R8600, read a known-empty channel
//     and record the answer frame in full.
//
//  6. AN ALL-FF RECORD ALSO MEANS EMPTY — WithEmptyReplyAllFF. Recorded
//     SEPARATELY from entry 5 and for a different reason: it is a claim about a
//     record-shaped answer, not about an FA, and one capture cannot establish
//     both. The one place FF appears with meaning in the guide is the clear
//     form's "(5): 'FF'"; that an ANSWER full of FFs means the same is not
//     stated. The answer this fake sends is 37 bytes — the shortest accepted
//     length, which invents the least — and its mode byte, FF, deliberately
//     selects no layout. Matrix entry: icr8600-empty-reply-ff.
//     LIFT (Stage W): clear a channel on a real IC-R8600 with the documented
//     clear form, read that channel, and record whether the answer is FA or an
//     all-FF record.
//
//  7. THE CLEAR FORM IS REFUSED. Refusing it is a TIER POLICY, not a fact about
//     the receiver: the form is printed in the guide's own words and a real
//     IC-R8600 presumably honours it. This project ships no erase builder, no
//     gate admission and no consent path for erase, so a fake that accepted one
//     would let an erase reach the wire in a test and pass. Group 0102 is
//     additionally excluded by the printed form itself ("You cannot specify
//     group '0102'"), which this fake need not encode because it refuses the
//     whole form.
//     LIFT: none — this entry retires when the project decides to ship an
//     erase path, not when a radio is observed.
//
//  8. THE MEMORY NAME'S CHARACTER CODES. PDF p.11 (folio 10)'s "● Character
//     entries" table gives the MEMORY NAME row's selectable characters — which
//     count out as exactly the 95 printable ASCII glyphs, space and ";" and "|"
//     included — and a total character number of 16, and PRINTS NO CODE FOR ANY
//     OF THEM. One ASCII byte per character is this package's reading, chosen
//     because ASCII is the only encoding the same guide names for any text
//     field at all (the D-STAR call-sign fields of command 20 03) and because
//     the listed repertoire maps one-to-one onto printable ASCII with nothing
//     left over. It is an inference from a different command's annotation.
//     Matrix entry: icr8600-name-charset-codes.
//     LIFT (Stage R): set a memory name on a real IC-R8600 containing one
//     character from each glyph class and read the 16 name bytes.
//
//  9. THE MEMORY NAME'S PAD BYTE (0x20). Graded SEPARATELY from entry 8: the
//     field's WIDTH is documented (16 cells drawn, "Total character number" 16)
//     and what fills the cells a shorter name leaves over is not. Space was
//     chosen over a null byte because "(space)" is explicitly one of the
//     selectable characters, so it is certainly a value the field can hold,
//     whereas 00 is not in the listed repertoire at all. Matrix entry:
//     icr8600-name-pad.
//     LIFT (Stage R): set a three-character memory name on a real IC-R8600 and
//     read all 16 name bytes.
//
//  10. THE DEFAULT IMAGE — WHICH CHANNELS ARE OCCUPIED. Eight channels in group
//     0, one per declared layout with both NXDN wire codes present. The guide
//     says nothing about how many channels an IC-R8600 ships occupied, or
//     which. The FIELD VALUES within those records are not invented — each is a
//     value its own printed domain admits — but the choice to have any occupied
//     channels at all, and these eight, is this package's.
//     LIFT: none needed; a consumer that wants a different image seeds it with
//     WithRecord and WithEmpty, which is what those Options are for.
//
//  11. THE SHORT SET IS REFUSED BY DEFAULT. The note beneath the record diagram
//     on PDF p.12 says a short set IS accepted for FM and Digital modes, "and
//     the default value is applied to the omitted items" — so the ACCEPTANCE is
//     documented and THE DEFAULTS ARE NOT, for any omitted byte of any tail.
//     Filling them in would invent up to eight values and then serve them back
//     as though a receiver had supplied them, so the fake refuses, and
//     WithShortSetsAccepted takes the fill byte FROM THE CALLER so that this
//     package never chooses one. This program always sends the full layout for
//     the mode in any case. Matrix entries: icr8600-short-set,
//     icr8600-tail-templates.
//     LIFT (Stage W): write an FM channel on a real IC-R8600 with the head
//     only, read it back, and record the tail values the receiver supplied.
//
//  12. THE FRAME-LENGTH CAP (256 body bytes). The guide states no such limit.
//     The cap is a property of a reader that must not grow without bound on a
//     line that has come up mid-frame; it is not a claim about the receiver.
//     The longest legitimate body this fake can receive is 53 bytes.
//     LIFT: none — this is a property of this reader.
//
//  13. THAT A MOVED RECEIVER ANSWERS ONLY ON ITS NEW ADDRESS. The default 96h is
//     printed and labelled "Receiver's default address", and PDF p.3
//     ("Preparing") documents that the address is set in Set mode — but the
//     admissible range is not printed here, and neither is the behaviour at the
//     old address. WithRadioAddress models the reading this program depends on:
//     it ships no --civ-address flag, so a moved receiver is unreachable by it
//     and times out. Matrix entry: icr8600-address-move.
//     LIFT (Stage R): move a real IC-R8600 to a non-default address, confirm
//     19 00 at 96h times out, and confirm it answers at the new address.
//
//  14. THE ECHO DEFAULT (off). Two per-port echo-back settings are printed
//     (PDF p.7: 1A 05 0094 front USB, 1A 05 0096 rear USB) and NEITHER DEFAULT
//     IS. Off is this fake's choice; WithEcho(true) turns on a bus echo that
//     repeats every complete frame VERBATIM, including frames addressed to
//     another radio, which is what a consumer's byte-identity echo suppression
//     must survive. Matrix entry: icr8600-echo-default.
//     LIFT (Stage R): factory-reset an IC-R8600, send 19 00 over each USB
//     port, and record whether the sent frame comes back.
//
//  15. THE TRANSCEIVE DEFAULT (off). Transceive exists and is settable
//     (1A 05 0092, "00=OFF, 01=ON"), and the guide prints no factory default,
//     instead telling the user to set it in Set mode. Off is this fake's
//     choice; WithTransceiveBroadcasts turns the flood on. Matrix entry:
//     icr8600-transceive-default.
//     LIFT (Stage R): factory-reset an IC-R8600, open a session without
//     writing anything, and record whether unsolicited frames arrive.
//
//  16. EVERY COMMAND BUT 19 00 AND 1A 00 IS REFUSED. Several of the refused ones
//     are real IC-R8600 commands the guide documents at length — 1A 05 heads
//     two full pages of set-mode items, 1A 0B 00 is the programmable scan-start
//     record, 1A 11 reads the CI-V connection terminal, 18 01 is the power-on
//     the guide's own worked example illustrates — so refusing them is this
//     FAKE's tier policy, not a fact about the receiver. It exists so that a
//     consumer that sent one would see it refused rather than quietly served.
//     LIFT: none — this entry retires when the tier decides to send more
//     commands, not when a radio is observed.
//
// # Hardware status
//
// UNVERIFIED. No IC-R8600 has ever been asked anything by this project. Every
// byte in this package is derived from printed documentation alone, and nothing
// this fake does is evidence about a real receiver.
package fakeicr8600
