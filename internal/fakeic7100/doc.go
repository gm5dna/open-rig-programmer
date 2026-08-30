// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7100 is a fake IC-7100 on the far end of a pipe: a CI-V
// transceiver that answers frames, holds a memory image, and can be told to
// take the other reading of every place the manual leaves something open.
//
// It is the test double the IC-7100 tier's own layers run against — the
// transport engine, the driver, and any --fake or demo path that reaches a rig
// — the role internal/fakeradio plays for the FT-710 and internal/fakeic9700
// for the IC-9700.
//
// # THE HARD RULE
//
// This package imports THE STANDARD LIBRARY AND NOTHING ELSE. Not
// core/civ/ic7100, not its profile, not its record layout, not its golden
// vectors, not its field ledger, not core/driver/ic7100, not core/spec, not
// core/codeplug, not core/transport, and not another fake. Not a constant, not
// a type, not a test helper. imports_test.go proves it, walking this directory
// and every directory beneath it, with vacuity guards and its own red proof,
// and it landed before any of the code below.
//
// The rule is not tidiness. A fake exists to be the OTHER witness in a test:
// the driver says what it believes the radio will say, the fake says what it
// believes the radio will say, and the test is worth something only because the
// two beliefs were formed separately. Let this package import the dialect it is
// tested against and the two witnesses become one — a systematic misreading of
// the record would agree with itself end to end and every test would go green
// while proving nothing.
//
// IT IS A SIBLING OF THE DIALECT, NOT A REFACTOR OF IT. Where this package and
// core/civ/ic7100 agree, they agree because two readings of one document landed
// in the same place, which is evidence. Where one of them imported the other,
// agreement would be a tautology and the evidence would be worth nothing.
//
// # WHERE THIS FAKE'S KNOWLEDGE CAME FROM
//
// Two kinds of fact, from two artefacts, and no third place. PROVENANCE.md
// names them.
//
// The WIRE facts are printed on PDF p.361 (folio 20-2), "◇ Data format", and
// PDF p.364 (folio 20-5), the command table: the FE FE … FD frame, the 88 and
// E0 addresses, the FB and FA acknowledgement codes, 19 00 and 1A 00, and the
// long preamble run the power-ON example on folio 20-4 requires. They are in
// parser.go, and none of them is a record secret.
//
// The RECORD facts come from two independent transcriptions of the one printed
// diagram on PDF p.375 (folio 20-16) — one carrying each field's meaning and
// values, one carrying each field's measured position. The derivation from
// those two to a 114-byte data area, a 3-byte address and a 111-byte record is
// written out IN FULL at the top of records.go, including the three places
// where the artefacts disagree with themselves and this package had to choose.
//
// NO IC-7100 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Every byte here
// descends from rasterised pages of one PDF, read by eye by agents who never
// opened this repository.
//
// # WHAT IT ANSWERS
//
// Frames addressed to its own address, and nothing else. Its answers name
// itself as from, and name the REQUESTER as to — index (3) of the printed
// frame is the "Controller's default address", a default on a bus the manual
// says may carry up to four CI-V devices, so the answer follows the frame
// rather than assuming E0.
//
//	19 00                                          -> 19 00 and an identity token
//	1A 00, an address, nothing more, occupied      -> 1A 00, the address, 111 record bytes
//	1A 00, an address, nothing more, unoccupied    -> FA (or an all-FF record; entry 2)
//	1A 00, an address, a whole record              -> FB, and the record stored
//	  … the same, under WithNoSetAnswer            -> nothing, and the record stored
//	1A 00, an address, a record of another length  -> FA
//	1A 00, a record whose transmit block differs   -> FA (entry 6)
//	1A 00, an address outside banks A-E / 1-99     -> FA (entry 10)
//	1A 00 in the printed clearing form             -> FA (entry 12)
//	any other command, addressed to this radio     -> FA (entry 13)
//	a frame addressed ANYWHERE ELSE                -> nothing at all (entry 11)
//
// The last line is the one that matters most. A radio at a different address
// never hears the frame, and the controller times out; a fake that answered NG
// instead would make the driver's timeout branch untestable, because nothing
// would ever fail to answer.
//
// It tolerates leading noise and any length of preamble run, and it records
// every frame it received — including the ones it ignored — for Transcript.
//
// # THE ASSUMED REGISTER
//
// Everything this package does that the artefacts do not print, with the lift
// that would retire each entry. A lift is per model and per firmware: nothing
// observed of any other radio, and nothing this fake does, is evidence about an
// IC-7100.
//
// Entries 1 to 10 carry the register names the IC-7100's own capability matrix
// assigns them, so that this fake's assumption and the profile's assumption
// about the same open question are retired by one capture rather than two.
// Entries 11 to 14 are this fake's own and have no matrix name.
//
//  1. A 1A 00 READ OF AN UNOCCUPIED CHANNEL IS ANSWERED FA
//     (ic7100-empty-channel-fa). The document describes the write-side clearing
//     form and never states what a READ of a cleared or never-written channel
//     returns. FA is this fake's choice, made because a driver needs SOME
//     distinguishable answer for "nothing there".
//     LIFT: clear channel A-099 from the front panel, read it with 1A 00, and
//     record the answer frame.
//
//  2. AN ALL-FF RECORD ALSO MEANS EMPTY (ic7100-all-ff-record). Recorded
//     separately from entry 1 because one capture cannot establish both. The
//     document's only use of FF as an emptiness marker is on the WRITE side,
//     and FF appears elsewhere in the chapter with unrelated meanings.
//     WithAllFFEmptyRecord builds the radio that answers this way.
//     LIFT: read a cleared channel that answers with a full-length record and
//     record whether every byte is FF.
//
//  3. THIS RADIO DOES NOT ECHO (ic7100-echo-default). The manual has no
//     echo-back setting anywhere: a search of the whole document for "echo"
//     returns nothing, and the CI-V set-mode group lists four items, none of
//     them an echo. But its [REMOTE] jack is a shared bus, on which echo would
//     be a property of the wiring rather than a setting, and the document never
//     says so either way. WithEcho builds the radio that echoes.
//     LIFT: send 19 00 to an IC-7100 over USB1 and record whether the
//     transmitted frame comes back before the answer.
//
//  4. THE IDENTITY TOKEN'S VALUE (ic7100-id-reply-value). The command table's
//     Data column for 19 00 is BLANK where every other row carries a value
//     range or a page reference, so the reply is undocumented. The default
//     DE AD was chosen precisely so that no reader could mistake it for a fact
//     about an IC-7100, and WithIdentityToken exists so a consumer can pin a
//     different one and prove its driver records whatever it gets rather than
//     matching a value.
//     LIFT: send 19 00 to an IC-7100 at 88h and record the answer bytes.
//
//  5. THE TRANSCEIVE BROADCAST FORM, to=00 (ic7100-broadcast-address-form).
//     This document NEVER PRINTS 00 as an address value: its frame diagrams
//     show only the point-to-point pair 88/E0, and the set-mode page describes
//     CI-V Transceive without giving the frame. WithTransceiveBroadcasts emits
//     the form a controller's address filter is built for; the frame's CONTENT
//     is arbitrary and asserts nothing. WithAddressedFlood is the same claim
//     for to=E0, and is a separate option because the two species exercise
//     different code in a controller.
//     LIFT: turn the dial on an IC-7100 with transceive ON and capture the
//     unsolicited frame's to byte.
//
//  6. THE TRANSMIT DUPLICATE MUST MATCH, AND A SHORT SET IS REFUSED
//     (ic7100-tx-block-mandatory). The printed NOTE says "The same data as
//     (5)-(51) are stored in 5f-51f" and "We recommend that you set the same
//     data as (5)-(51)" — a description and an advisory, not a stated rule.
//     The document never says what a radio does with a set whose blocks differ,
//     nor whether a set that stops short of the full record is accepted at all.
//     This fake refuses both; WithUnequalTransmitBlockAccepted and
//     WithShortSetsAccepted build the radio that does not.
//     LIFT: write a channel with the transmit block deliberately differing from
//     the receive block and split OFF, read it back, and record which block the
//     radio kept; and send a set that stops after (51) and record the answer.
//
//  7. THE WIRE ORDER PAST THE DUPLICATE (ic7100-wire-order). The printed field
//     indices and the measured positions part company after (51): the group
//     printed 5f-51f is measured at data-area bytes 52-98, and the group whose
//     printed index begins at (52) is measured beginning at byte 99. This
//     package follows the MEASURED positions, so the name occupies data-area
//     bytes 99-114 and the transmit duplicate 52-98. Both artefacts record the
//     divergence; neither reconciles it.
//     LIFT: store a known 16-character name in a scratch channel from the front
//     panel, read that channel with 1A 00, and record the offset, counted from
//     the first byte of the data area, at which the name begins.
//
//  8. THE RECORD'S LENGTH (ic7100-record-length). The manual prints no total
//     for this record anywhere; 114 data-area bytes and 111 record bytes are a
//     derivation from the printed field-group widths, set out term by term in
//     records.go and re-done from the group table by the tests. The near miss
//     is named there too: taking the diagram bar's own (52)~(60) label at face
//     value gives 107 and 104, which is where a text-only reading lands.
//     WithAcceptedRecordLength builds a radio of another length so that a
//     driver's fingerprint can be tested against one.
//     LIFT: read one occupied channel from an IC-7100 with 1A 00 and count the
//     answer's data bytes.
//
//  9. THE READ-REQUEST FORM (ic7100-read-request-form). This fake treats
//     1A 00 followed by the three address bytes and nothing else as a read.
//     The document prints ONE layout for 1A 00 — the complete record — and
//     introduces it without distinguishing a read from a write; the command
//     table names both directions and points at that same single layout. No
//     shortened read form is printed anywhere in the chapter.
//     LIFT: send FE FE 88 E0 1A 00 01 00 01 FD to an IC-7100 and record whether
//     a record answer comes back.
//
//  10. THE SPECIAL CHANNELS ARE REFUSED (ic7100-special-bank-byte). The field
//     legend names ten further channel codes — 0100-0105 programmed scan edges
//     and 0106-0109 call channels — but the ONLY thing the document says about
//     bank byte (1) is its own legend, "01: A … 05: E". It never says what (1)
//     carries when the channel is one of those ten, and the clearing block
//     omits (1) entirely. This fake will not invent a bank byte, so it refuses
//     every address whose channel is outside 0001-0099, and WithSlot will not
//     seed one. That refusal is the assumption: a real IC-7100 evidently HAS
//     those channels.
//     LIFT: select programmed scan edge 0100 and call channel 0106 from an
//     IC-7100's front panel, read each with 1A 00, and record byte (1).
//
//  11. SILENCE FOR A FRAME ADDRESSED ELSEWHERE. Both printed frame diagrams
//     carry a to byte, and an address byte exists to single out one radio on a
//     bus the manual says may hold four, so a radio ignoring what is not
//     addressed to it is a very short inference — but it IS an inference,
//     because no printed line says "and otherwise it says nothing".
//     LIFT: put a frame addressed to another CI-V address to a real IC-7100 on
//     a shared bus and observe the silence.
//
//  12. REFUSING THE PRINTED CLEAR FORM. PDF p.375's "About clearing operation"
//     block prints the clear — "(2), (3): Memory channel 0 to 99 / (4) : FF /
//     (5) or later: None" — so a real IC-7100 evidently honours something of
//     that shape. This fake refuses it, in BOTH its printed readings (with the
//     bank byte and without, since the block omits field (1)), because THIS
//     TIER SENDS NO CLEAR: there is no builder, no gate admission, and every
//     Icom driver gives FieldErase no write support. A fake that accepted
//     traffic the tier never emits would be simulating a radio nobody is
//     driving, and would let a driver bug that reached for a clear pass
//     unremarked.
//     LIFT: the day this tier sends a clear, this entry goes and the behaviour
//     changes with it — at which point the block's own contradictions (channel
//     "0 to 99" against the field legend's 0001-0099, and the missing bank
//     byte) have to be adjudicated against a radio first.
//
//  13. REFUSING EVERY OTHER COMMAND. 1A 01 (band stacking register), 1A 05
//     (the set-mode head), 0B (memory clear), 18 01 (power on) and the rest of
//     the chapter's table are answered FA. At least three of them are
//     demonstrably real commands with their own printed diagrams. Refusing them
//     is tier policy, not a fact about the radio.
//     LIFT: the tier learning to send any of them.
//
//  14. THE FRAME-BODY CAP. The document states no frame-length limit. The cap
//     in parser.go is a property of a reader that must not grow without bound
//     on a line that has come up mid-frame; it is set far above the longest
//     frame this radio can be asked for, the 121-byte complete-record set, and
//     an over-long run is DROPPED rather than answered, because a radio that
//     never saw a whole frame has nothing to reply to.
//     LIFT: put an over-long run to a real IC-7100 and record what it does.
//
// # WHAT IS NOT IN THAT REGISTER
//
// The framing itself — FE FE, FD, 88, E0, FB, FA, the two frame orders, the
// long preamble run, and the 19 00 and 1A 00 command rows — is PRINTED, on
// PDF p.361 (folio 20-2), p.363 (folio 20-4) and p.364 (folio 20-5). So are the
// bank codes 01-05, the channel range 0001-0099, and every field-group width
// the record's length is derived from. Those are facts, cited in parser.go and
// records.go where they are used, and they do not belong in a register of
// assumptions. The DERIVATION built on them is entry 8; the widths themselves
// are not.
//
// Nor is the empty image an assumption. No channel is occupied until a test
// seeds one with WithSlot or writes one, because the manual prints each field's
// permitted VALUES and never a shipped default — so there is nothing to source
// a factory record from, and inventing one would put bytes in front of anybody
// who renders a fake rig's memories.
//
// Nor is WithNoSetAnswer one. It is a TEST LEVER, and the only option in this
// package that is not the other reading of an open page. The radio hears an
// acceptable set, STORES it exactly as it always does, and then says nothing
// at all — no FB, and no FA either, because nothing was refused. That models a
// LOST ACKNOWLEDGEMENT ON THE LINK, not an IC-7100 declining to acknowledge, so
// no capture from a radio could settle it and there is nothing here to lift.
//
// It is in this package because of the tier's WRITE QUARANTINE RULE, which the
// transport engine states and this fake must be able to put a driver through
// (core/transport, "Command classes are stated, not inferred"): a memory set is
// an acknowledged write, transmitted EXACTLY ONCE and never retransmitted when
// the acknowledgement fails to arrive, with an unconditional post-write
// quarantine drain afterwards whatever the outcome. A radio that always answers
// cannot take a driver down that branch. This is the radio that hears one set
// frame and then goes quiet; entry 11's silence is its read-path counterpart,
// and strands a read the same way.
//
// Because the set IS stored, Slot and Transcript still report it, which is how
// a test tells a lost acknowledgement from a write that never landed. Nothing
// else changes: reads, 19 00 and every refusal are answered as before.
//
// # HARDWARE STATUS: UNVERIFIED
//
// Nothing here has been put to a radio, and until something is, every entry in
// the register above stands.
package fakeic7100
