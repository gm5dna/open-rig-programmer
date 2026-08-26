// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic705 simulates an Icom IC-705's CI-V behaviour over an in-memory
// serial connection (Radio.Port()). It is the test double the IC-705's own
// layers run against — the transport framing, the CI-V engine, core/driver/ic705
// and the CLI's --fake mode — the role internal/fakeradio plays for the FT-710
// and internal/fakedx101 for the FTdx101 pair.
//
// # The wire this fake speaks
//
// A frame is
//
//	FE FE <to> <from> <cn> [<sc>] <data> FD
//
// A LEADING EXTRA FE IS PADDING and is tolerated, so the opening run may be
// longer than two. FD terminates, and nothing else does.
//
// This radio's default address is A4 and the controller's is E0. THE FAKE
// ANSWERS ONLY FRAMES WHOSE `to` IS A4, and every answer it sends is from A4 to
// E0. FB is the OK code and FA the NG code, each carried alone in a six-byte
// frame: FE FE E0 A4 FB FD, FE FE E0 A4 FA FD.
//
// Two commands are answered:
//
//   - 19 00, read the transceiver ID. The request carries no data area. The
//     answer's payload is this fake's own invention — register entry 7.
//   - 1A 00, send/read memory contents. Its data area opens with a four-byte
//     address: two packed-BCD bytes of memory group, then two of memory
//     channel. The address alone is a READ; the address followed by a record is
//     a SET.
//
// THE RECORD-LENGTH RULE, which this fake enforces without exception: a 1A 00
// SET whose record — the data area after those four address bytes — is not
// RecordLen (111) bytes is refused with FA. Never accepted, never truncated,
// never padded.
//
// Group 0100 is the call-channel group and holds channels 0000-0003; groups
// 0000-0099 hold channels 0000-0099. Any other address is refused with FA, as
// is every command that is not 19 00 or 1A 00.
//
// # What this fake knows about a record: nothing
//
// A record is 111 OPAQUE BYTES. This package stores them, serves them and
// compares them; it has no field table, no encoder, no vocabulary and no
// opinion about what any byte means. That is deliberate and it is the point:
// the layer under test is the one with the field table, and a fake that shared
// its reading of the diagram could not disagree with it.
//
// The ONE number this package does derive is 111 itself, and it derives it
// arithmetically rather than semantically, from the two independent
// transcriptions committed under core/civ/ic705/testdata:
//
//	transcription-b.csv     widths 2+2+1+5+2+1+1+1+3+3+3+1+3+8+8+8+47+16 = 115
//	geometry-witness.csv    measured byte positions 1 to 115, same field extents
//
// Of those 115 positions, 1-2 are the memory group and 3-4 the memory channel —
// the address the command carries in front of the record — so a record is
// 115 - 4 = 111. The two transcripts were produced by different quarantined
// readings of the same page and AGREE ON EVERY FIELD EXTENT AND ON THE TOTAL.
// TestRecordLenIsTheDiagramsOwnArithmetic pins the subtraction so that a
// mistyped constant fails here rather than at a driver three layers away.
//
// # The STOPs this package inherits and does not resolve
//
// Both transcripts record, independently, that the printed diagram contradicts
// its own measured geometry in two places: the eighteenth field prints the
// indices 6 to 52 in black-filled circles where the running count gives byte
// positions 53-99, and the nineteenth prints 53 to 68 where the running count
// gives 100-115. Both readings record printed index and measured position side
// by side and reconcile neither. A third STOP is transcription B's: two adjacent
// fields carry the identical printed label over different byte ranges.
//
// NONE OF THE THREE IS WORKED AROUND HERE, and none of the three has to be,
// because the section above is the reason: a fake that knows no field cannot be
// wrong about where a field starts. It is recorded because it decides one thing
// that would otherwise look arbitrary — why BlankRecord and DefaultImage put
// nothing recognisable at any named position. Placing a memory name, say, would
// require choosing between the printed index and the measured position, which
// is exactly the choice both transcripts refused to make.
//
// # The hard rule: NOTHING project-internal
//
// fakeic705 MUST NOT import any package of this project — not core/civ, not
// core/civ/ic705, not core/driver/ic705, not core/codeplug, not core/spec, not
// internal/fakeradio and not internal/fakedx101. Standard library only, in
// every non-test file, in this directory AND every directory beneath it.
//
// This is not a style preference, and the reasoning is internal/fakeradio's
// verbatim: if this fake reused the production codec, a systematic bug in that
// codec — an off-by-one in a field offset, a validation rule subtly wrong —
// would be applied identically on both sides of every "send a command, check
// the reply" test this project runs. The bug would never surface. The fake
// would misbehave in exactly the way the buggy codec expects, and every
// end-to-end test would pass anyway. Two independent implementations of one
// protocol, checked against each other — and against expectations recomputed by
// hand in tests, never by calling this package's own builders — is what makes
// that class of bug visible. It bites twice as hard here, because the two
// implementations are reading a diagram whose own indices disagree with its own
// geometry.
//
// TestNoCoreImports (imports_test.go) enforces it with a go/parser scan, and
// THAT SCAN WALKS SUBDIRECTORIES. This package has no subdirectory today; the
// fence lands recursive anyway, so that anything added beneath it later arrives
// inside a fence rather than in front of one, and
// TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory proves it
// would bite before any such directory exists.
//
// # A SIBLING of internal/fakeradio and internal/fakedx101, not a refactor
//
// This package duplicates a good deal of both: the pipe-and-goroutine Radio,
// the bounded reassembler, the per-command handlers, the Image contract, the
// options, the import fence. That duplication is deliberate and it is not going
// to be factored into a shared "fake core" package.
//
// Two reasons, both load-bearing. The first is mechanical: a shared helper
// package would be a project-internal import, which the hard rule above forbids
// in all of the fakes, so the only way to share code would be to abandon the
// property that makes any of them worth having. The second is that three radios
// agreeing on a structure is a fact about those three radios, not a shared
// definition — and here it is not even the same protocol. The FT-710's and the
// FTdx101's wire is ASCII CAT, terminated by ';', with parameters spelled in
// digits. This one is binary CI-V, addressed, terminated by FD, with packed
// BCD. What this package shares with fakedx101 is a SHAPE and nothing else, and
// every byte below was written from the frame grammar rather than adapted from
// a sibling's code.
//
// Where this fake's behaviour differs from its siblings', the difference is a
// decision with a reason, stated at the code that implements it. The three that
// a reader of internal/fakedx101 will notice:
//
//   - NEW() SEEDS NOTHING. fakedx101's New falls back to DefaultImage(); this
//     one starts empty, and DefaultImage is opt-in through WithFactoryImage.
//     The design's probe treats an all-NG inventory search as a real and
//     expected case — an unprogrammed radio — and a default image would put
//     records in slots no test asked for, competing silently with whatever that
//     test seeded. (New)
//   - THERE IS A MISBEHAVIOUR HOOK. AnswerNextReadWithAddress makes the next
//     read answer under an address that is not the one asked for. Neither
//     sibling has an equivalent, because neither sibling's protocol repeats the
//     slot address in the answer — CI-V does, so a driver can and must check
//     it, and a fake that cannot misbehave cannot prove the driver checks.
//     (state.go)
//   - THERE ARE TWO FLOODS, not one. See register entry 9 and WithNeverQuiet.
//
// # What this fake deliberately does NOT model
//
// FAULTS. internal/fakeradio carries a scripted misbehaviour set —
// FaultDropReplies, FaultGarbleReply, FaultSpuriousFrame, FaultDelayedRejection,
// FaultDelayedReply, FaultDisconnect, FaultChunkedReplies — and this package
// carries none of them, on internal/fakedx101's recorded reasoning: those faults
// exercise the transport engine's timeout, resync and chunk-reassembly
// behaviour, which is model-independent and already covered. WithLatency is
// kept, because it is not a fault: it is the knob Close's promptness is proven
// against. The one misbehaviour that IS modelled,
// AnswerNextReadWithAddress, is not in that class at all — it is a
// PROTOCOL-SPECIFIC lie about which channel an answer is about, and nothing
// else in this repository can produce one.
//
// EVERY OTHER COMMAND. 1A 01 (band stacking register), 1A 05 (set mode), the
// clear forms, the transceive sets: all refused with FA. The design's own gate
// admits none of them, so nothing in this project sends one, and implementing
// them would be inventing wire behaviour that no test could check.
//
// TIMING. Every reply is near-instant unless WithLatency says otherwise. No
// IC-705 timing has ever been observed by this project, so there is nothing to
// model.
//
// # The ASSUMED register
//
// NO IC-705 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Not one frame has
// been put to one, and nothing below has been observed. The two artefacts this
// package rests on are transcriptions of a printed page, and a transcription of
// a page says what a command's frame LOOKS like, never what a radio DOES at the
// protocol's edges — which is most of what a fake has to decide.
//
// Every place this fake had to guess is listed here, in one place, with the one
// capture that would lift it, so that a reviewer — or the first real IC-705
// session — has a single list to work from rather than a source-wide comment
// hunt. Each entry also appears as an inline comment beside the code that
// implements it.
//
// EVERY ENTRY BELOW IS A CLAIM ABOUT THIS FAKE, CONSUMED BY TESTS. It is not a
// claim about an IC-705, and nothing downstream may read it as one: what an
// entry says is "this simulator behaves so, and the tests that rest on it are
// resting on a decision".
//
//  1. AN UNWRITTEN SLOT ANSWERS FA. A read of a grammatically valid, in-range
//     address this fake holds no record for draws the NG frame, which is the
//     only unattributed refusal this protocol has. The manual documents the
//     read command and never says what a radio answers when the channel is
//     empty; silence and a zero-filled record are both conceivable and would
//     both change what an inventory walk means.
//     STAGE R LIFTS IT WITH: one 1A 00 read of a memory channel that radio has
//     never had written, with the port watched for a reply at all. An answer
//     rather than FA — or silence — moves this fake and the driver's
//     empty-slot interpretation together.
//     (parser.go: handleMemoryRead)
//
//  2. AN OUT-OF-RANGE ADDRESS ANSWERS FA, AND SO DOES A NON-DECIMAL BCD NIBBLE.
//     Group 0101, channel 0100 of a memory group, channel 0004 of the call
//     channel group, and a nibble above 9 anywhere in the four address bytes,
//     are all refused. The transcribed vocabularies are evidence of what the
//     RANGES are; nothing states what a radio does when addressed outside them,
//     and clamping, wrapping or ignoring are all things real firmware does.
//     The refusal is checked BEFORE the state is consulted, so an out-of-range
//     read is refused identically whether or not something has been seeded
//     there — which is itself a modelled decision, and the one
//     TestReadMemory_OutOfRangeAddressesAnswerNG holds in place.
//     STAGE R LIFTS IT WITH: one read each of 0101/0000 and 0100/0004 on a
//     radio, and one read carrying 0x0A in a group nibble.
//     (parser.go: decodeAddress, inRange)
//
//  3. A SET CREATES AN ABSENT SLOT. A 1A 00 set to a channel this fake holds
//     nothing for stores the record, exactly as it would over an existing one,
//     and answers FB. It has to be modelled because a driver has no other write
//     path, and a fake that demanded the channel exist first could not be
//     written to at all.
//     STAGE W LIFTS IT WITH: the first write trial — one 1A 00 set to a
//     verified-empty channel, then a read. NOTHING MAY BE WRITTEN TO A REAL
//     IC-705 WHILE ITS WRITE GUARD IS FALSE, AND IT IS FALSE.
//     (parser.go: handleMemorySet)
//
//  4. THE SOURCE ADDRESS IS NOT CHECKED, AND EVERY ANSWER GOES TO E0. A frame
//     addressed to A4 is answered whatever its `from` byte says, and the answer
//     is always addressed to the controller. On a bus with a second controller
//     this would be wrong twice over — the answer would go to the wrong
//     station. Nothing states what the radio does, and the alternative (answer
//     the sender) would make the fake's behaviour depend on a byte no test in
//     this project varies.
//     STAGE R LIFTS IT WITH: one read frame carrying a `from` byte that is not
//     E0, and the answer's own `to` byte recorded.
//     (parser.go: handleFrame)
//
//  5. A BYTE RUN THAT IS NOT A FRAME DRAWS SILENCE, AND SO DOES A FRAME FOR
//     ANOTHER STATION. A run with no two-byte preamble, or with no room for
//     both address bytes, carries no destination, so this radio cannot know it
//     was meant for it — and a bus with two radios on it would answer every
//     frame twice if either refused what it could not attribute. That reasoning
//     is sound and it is still an assumption: an FA to every malformed run is
//     equally conceivable, and would be noisier and easier to debug against.
//     STAGE R LIFTS IT WITH: one run of junk terminated by FD, and one
//     well-formed frame addressed to some other station, each with the port
//     watched.
//     (parser.go: handleFrame; parser_test.go: TestMalformedFramesDrawSilence)
//
//  6. THE ACCUMULATOR'S CAP AND RESYNC. Once more than maxAccumulatorBytes have
//     accumulated without an FD, this fake replies FA once and discards bytes
//     up to and including the next FD before resuming normal framing. NOT a
//     radio claim and NOT liftable by any capture: it is this package's own
//     bounded-input policy, inherited from fakeradio's, recorded here so that a
//     test relying on it knows what it is relying on.
//     (parser.go: reassembler; fakeic705.go: handleEvent)
//
//  7. THE TRANSCEIVER ID PAYLOAD IS INVENTED. The answer to 19 00 carries one
//     byte, A4, and it is this package's choice. Neither transcript carries an
//     ID value — both read the memory-record pages — and no IC-705 has been
//     asked. The design's own probe records the ID reply in diagnostics and
//     NEVER MATCHES IT, so any value is behaviourally equivalent and only its
//     honesty is at stake; A4 is chosen because it is the radio's own default
//     address and so cannot be mistaken in a capture for evidence about some
//     other radio. A CONSUMER THAT EVER MATCHES ON THIS VALUE IS TESTING THIS
//     PACKAGE'S INVENTION, and the match will be a false pass.
//     STAGE R LIFTS IT WITH: one 19 00 read of a real radio, the answer
//     recorded byte for byte. Record the port and the radio's configured
//     address with it.
//     (parser.go: transceiverIDPayload)
//
//  8. THE FIXTURE RECORDS' CONTENT IS INVENTED, AND IS ALL ZEROS. BlankRecord
//     and every record in DefaultImage are 111 zero bytes. The diagram
//     documents each field's vocabulary and its valid range and NEVER A SHIPPED
//     DEFAULT, so there is nothing to source a factory value from; zero is
//     chosen because it is the only fill that asserts nothing — a legal packed-
//     BCD digit, and what the diagram's one printed literal already prints.
//     WHAT IS NOT INVENTED IS THE SHAPE: DefaultImage's sparse, two-group,
//     call-channel-carrying inventory exists so that a walk has something to
//     find, something to skip and a group boundary to cross, and that is a
//     property of the fixture rather than a claim about any radio's memory.
//     STAGE R LIFTS IT WITH: a full 1A 00 sweep of a factory-condition radio —
//     which reports THAT radio's inventory, and one radio's inventory is not
//     the model's, so expect this entry to stay ASSUMED with better
//     placeholders rather than to retire.
//     (image.go: BlankRecord, DefaultImage)
//
//  9. UNSOLICITED TRAFFIC: THIS FAKE PUSHES NOTHING UNLESS ASKED TO, AND WHAT
//     IT PUSHES WHEN ASKED IS INVENTED. With no flood option a Radio writes to
//     the port only in reply to a frame that arrived on it, so a session with
//     transceive on and one with it off are byte-identical on the wire. The
//     assumption is not that an IC-705 is silent — it is that MODELLING IT AS
//     SILENT IS THE HONEST DEFAULT, because no IC-705 has been observed at all.
//     Which frames such a radio volunteers, on what triggers, in what order and
//     with what interleaving against a command in flight are four unknowns, and
//     an invented push is indistinguishable at the far end of a link from a
//     transport defect.
//     WithNeverQuiet, WithBroadcastEvery and WithAddressedFlood therefore emit
//     a frame this package MADE UP: FE FE <to> A4 1A 01 00 00 FD. Its shape is
//     chosen so that it cannot be mistaken for an answer this radio owes
//     anyone — 1A 01 is a different command that the design's gate refuses, and
//     transcription B records it as a separate diagram that is not a memory
//     record — and its two data bytes mean nothing whatever. A CONSUMER MAY
//     ASSERT ON ITS `to` BYTE, WHICH IS THE OPTION'S WHOLE POINT, AND MUST NOT
//     ASSERT ON ANYTHING ELSE IN IT.
//     THE TWO FLOODS ARE NOT INTERCHANGEABLE and the difference is that one
//     byte: broadcasts (to = 00) are dropped at a well-built adapter's framing
//     seam and never reach an engine, while the addressed flood (to = E0)
//     cannot be dropped on address alone and is the only kind that reaches a
//     drain and trips its cap. A test that reaches for the wrong one proves
//     nothing and passes.
//     STAGE R LIFTS IT WITH: one session with transceive enabled and the port
//     then watched — idle, and while the front panel is operated. Whatever that
//     radio pushes, and whatever provokes it, becomes this fake's model.
//     NOTHING MAY BE INVENTED MEANWHILE beyond what is admitted here, and an
//     observation of nothing at all is a result to record rather than a failed
//     capture.
//     (options.go: WithNeverQuiet, WithBroadcastEvery, WithAddressedFlood;
//     parser.go: buildUnsolicited; fakeic705.go: emit)
//
//  10. A WELL-FORMED FRAME WITH THE WRONG SHAPE DRAWS FA, NOT SILENCE. A frame
//     addressed to A4 carrying no command byte at all, a command byte with no
//     sub-command, a 19 00 carrying a data area, a 1A 00 whose data area is
//     shorter than four bytes: all refused. Each is a different malformation
//     and the protocol has one refusal to report them all with, so nothing
//     downstream can tell them apart — which is a property of the wire and not
//     of this fake, and is why a driver's diagnostics must record what it SENT
//     rather than infer from what came back.
//     STAGE R LIFTS IT WITH: one frame per shape, the reply recorded.
//     (parser.go: handleFrame, handleTransceiverID, handleMemoryContent)
//
//  11. AN ACCEPTED SET IS ACKNOWLEDGED WITH FB AND NOTHING ELSE FOLLOWS, AND A
//     READ ANSWER ECHOES THE ADDRESS IT WAS ASKED FOR. The first half is what
//     the OK code is for and is as close to stated as anything here gets; what
//     is assumed is that the acknowledgement is the WHOLE reply — no second
//     frame, no state change visible anywhere else, and in particular no
//     selection moved (this fake models no selected channel at all, because
//     nothing in front of it needs one). The second half is what makes
//     AnswerNextReadWithAddress meaningful: an honest answer carries the
//     requested address, so a mismatched one is a detectable lie.
//     STAGE W LIFTS THE FIRST WITH: one accepted set with the port watched for
//     any further frame before the next command is sent. STAGE R LIFTS THE
//     SECOND WITH: one read whose answer's address bytes are compared with the
//     request's.
//     (parser.go: handleMemorySet, buildMemoryAnswer, handleMemoryRead)
//
// # What is NOT in this register, and why
//
// The frame grammar itself, the two addresses, the two reply codes, the two
// commands, the record length and the group and channel vocabularies are
// MANUAL-DERIVED FACTS and are absent from the register deliberately, so that
// the absences read as decisions rather than as oversights:
//
//   - FE FE <to> <from> <cn> [<sc>] <data> FD, the tolerated extra FE, and FD
//     as the only terminator.
//   - A4 as this radio's default address and E0 as the controller's.
//   - FB as OK and FA as NG, each in a six-byte frame.
//   - 19 00 as the transceiver-ID read carrying no data area, and 1A 00 as
//     send/read memory contents opening with four packed-BCD address bytes.
//   - The 111-byte record length, which is the two transcripts' agreed 115
//     positions less the four the address occupies. The SUBTRACTION is this
//     package's; the 115 and the address's place in it are transcribed twice
//     over, independently.
//   - Groups 0000-0099 with channels 0000-0099, and group 0100 — the call
//     channel group — with channels 0000-0003.
//
// What a radio DOES when any of those is violated is a different question
// entirely, and every one of those answers is in the register above.
//
// # Hardware status
//
// UNVERIFIED. No IC-705 has ever been asked anything by this project — see
// PROVENANCE.md, which says the same thing from the artefacts' side and lists
// every byte this package invents.
package fakeic705
